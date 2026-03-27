import os
from typing import Annotated, Any, TypedDict, List
from dotenv import load_dotenv

# Load environment variables
load_dotenv()

from langchain_openai import ChatOpenAI
from langchain_tavily import TavilySearch
from langchain_core.messages import AIMessage, BaseMessage, HumanMessage, SystemMessage, ToolMessage
from langgraph.graph import StateGraph, START
from langgraph.prebuilt import ToolNode, tools_condition

# Verify API keys
if not os.environ.get("OPENAI_API_KEY"):
    print("Warning: OPENAI_API_KEY not found in environment variables.")
if not os.environ.get("TAVILY_API_KEY"):
    print("Warning: TAVILY_API_KEY not found in environment variables.")

# Define our State
class AgentState(TypedDict):
    messages: Annotated[List[BaseMessage], lambda x, y: x + y]


class ConversationMessage(TypedDict):
    role: str
    content: str


def build_system_prompt(query: str) -> str:
    normalized = query.lower()
    module_mode = (
        "find the latest security vulnerabilities" in normalized
        or "exposed assets" in normalized
        or "technology stack information for" in normalized
    )

    base_prompt = (
        "You are an expert pentester assistant helping with authorized security assessments. "
        "Your objective is to think like a skilled penetration tester: identify exposed assets, "
        "attack surface, likely technologies, security-relevant changes, public vulnerabilities, "
        "misconfigurations, weak points, and realistic avenues for further validation. "
        "Use the Tavily search tool aggressively when current external context would improve the answer. "
        "Prioritize recent, verifiable information and connect findings to practical security impact. "
        "Highlight what is confirmed vs inferred, mention useful hypotheses, and call out severity or risk "
        "when appropriate. Focus on reconnaissance and defensive assessment for authorized environments. "
    )

    if module_mode:
        return (
            base_prompt
            + "You are operating in automated recon module mode. "
            + "Produce a compact technical report optimized for later storage as evidence. "
            + "Prefer sections such as Attack Surface, Observed Technologies, Public Security Signals, "
            + "Likely Risks, and Next Validation Steps. Avoid filler, long introductions, and generic advice."
        )

    return (
        base_prompt
        + "You are operating in interactive ask-ai mode. "
        + "Answer directly, clearly, and with strong security intuition. "
        + "Be helpful to a human operator: explain why a finding matters, what to verify next, "
        + "and where uncertainty remains."
    )


def build_agent(model_name: str = None):
    if not model_name:
        model_name = "gpt-4o-mini"
    llm = ChatOpenAI(model=model_name, temperature=0)
    tavily_tool = TavilySearch(
        max_results=2,
        include_raw_content=False,
        search_depth="basic",
    )
    tools = [tavily_tool]
    llm_with_tools = llm.bind_tools(tools)

    def chatbot(state: AgentState):
        """The main LLM node that decides what to do."""
        messages = state["messages"]
        sys_msg = SystemMessage(
            content=build_system_prompt(messages[-1].content if messages else "")
        )

        if not messages:
            messages = [sys_msg]
        elif not isinstance(messages[0], SystemMessage):
            messages = [sys_msg] + messages

        response = llm_with_tools.invoke(messages)
        return {"messages": [response]}

    graph_builder = StateGraph(AgentState)
    graph_builder.add_node("chatbot", chatbot)
    graph_builder.add_node("tools", ToolNode(tools=tools))
    graph_builder.add_edge(START, "chatbot")
    graph_builder.add_conditional_edges("chatbot", tools_condition)
    graph_builder.add_edge("tools", "chatbot")
    return graph_builder.compile()


def convert_conversation_to_messages(
    history: List[ConversationMessage] | None,
    query: str,
) -> List[BaseMessage]:
    messages: List[BaseMessage] = []

    for entry in history or []:
        role = entry.get("role")
        content = entry.get("content", "")
        if not content:
            continue
        if role == "assistant":
            messages.append(AIMessage(content=content))
        elif role == "user":
            messages.append(HumanMessage(content=content))

    if not messages or not isinstance(messages[-1], HumanMessage) or messages[-1].content != query:
        messages.append(HumanMessage(content=query))

    return messages


def build_initial_state(
    query: str,
    history: List[ConversationMessage] | None = None,
) -> AgentState:
    return {"messages": convert_conversation_to_messages(history, query)}

# We defer initialization so that we can pick up dynamic environment variables
# when process_query is called per-request.
def get_agent(model_name: str = None):
    return build_agent(model_name)

def process_query(
    query: str,
    model_name: str = None,
    history: List[ConversationMessage] | None = None,
) -> str:
    """
    Takes a user query, runs it through the LangGraph agent,
    and returns the final string response.
    """
    agent = get_agent(model_name)
    initial_state = build_initial_state(query, history)

    # Run the graph
    # stream_mode="values" yields the full state at each step
    final_state = None
    for event in agent.stream(initial_state, stream_mode="values"):
        final_state = event

    # Extract the final message content
    if final_state and "messages" in final_state and final_state["messages"]:
        last_message = final_state["messages"][-1]
        return last_message.content

    return "Error: Could not generate a response."

def extract_text_content(content: Any) -> str:
    if isinstance(content, str):
        return content
    if isinstance(content, list):
        parts: List[str] = []
        for item in content:
            if isinstance(item, str):
                parts.append(item)
            elif isinstance(item, dict):
                text = item.get("text")
                if isinstance(text, str):
                    parts.append(text)
        return "".join(parts)
    return ""


def summarize_event_data(data: dict[str, Any] | None) -> dict[str, Any]:
    if not data:
        return {}

    summary: dict[str, Any] = {}

    chunk = data.get("chunk")
    if chunk is not None:
        text = extract_text_content(getattr(chunk, "content", chunk))
        if text:
            summary["text"] = text

    output = data.get("output")
    if output is not None:
        if isinstance(output, ToolMessage):
            summary["output"] = extract_text_content(output.content)
        else:
            summary["output"] = str(output)[:500]

    input_data = data.get("input")
    if input_data is not None:
        summary["input"] = str(input_data)[:500]

    return summary


async def stream_query_events(
    query: str,
    model_name: str = None,
    history: List[ConversationMessage] | None = None,
):
    agent = get_agent(model_name)
    initial_state = build_initial_state(query, history)

    yielded_text = ""
    yielded_searching = False

    async for event in agent.astream_events(initial_state, version="v2"):
        name = event.get("name")
        event_name = event.get("event")
        data = event.get("data", {})

        yield {
            "type": "langgraph",
            "event": event_name,
            "name": name,
            "data": summarize_event_data(data),
        }

        if event_name == "on_chain_start" and name == "LangGraph":
            yield {"type": "status", "status": "thinking"}

        if event_name == "on_tool_start" and not yielded_searching:
            yielded_searching = True
            yield {"type": "status", "status": "searching"}

        if event_name == "on_chat_model_stream":
            chunk = data.get("chunk")
            text = extract_text_content(getattr(chunk, "content", chunk))
            if text:
                yielded_text += text
                yield {"type": "chunk", "content": text}

    if not yielded_text:
        final_text = process_query(query, model_name, history)
        if not final_text:
            yield {"type": "error", "error": "Could not generate a response."}
            return
        yield {"type": "chunk", "content": final_text}

    yield {"type": "done"}

# For testing locally
if __name__ == "__main__":
    import sys
    test_query = "What are the latest features in Go 1.22?"
    if len(sys.argv) > 1:
        test_query = sys.argv[1]

    print(f"Query: {test_query}")
    print("Thinking...")
    response = process_query(test_query)
    print(f"\nResponse:\n{response}")
