import os
from typing import Annotated, TypedDict, List
from dotenv import load_dotenv

# Load environment variables
load_dotenv()

from langchain_openai import ChatOpenAI
from langchain_tavily import TavilySearchResults
from langchain_core.messages import BaseMessage, HumanMessage, AIMessage, SystemMessage
from langgraph.graph import StateGraph, END, START
from langgraph.prebuilt import ToolNode, tools_condition

# Verify API keys
if not os.environ.get("OPENAI_API_KEY"):
    print("Warning: OPENAI_API_KEY not found in environment variables.")
if not os.environ.get("TAVILY_API_KEY"):
    print("Warning: TAVILY_API_KEY not found in environment variables.")

# Define our State
class AgentState(TypedDict):
    messages: Annotated[List[BaseMessage], lambda x, y: x + y]

# We defer initialization so that we can pick up dynamic environment variables
# when process_query is called per-request.
def get_agent():
    # Initialize the LLM
    llm = ChatOpenAI(model="gpt-4o-mini", temperature=0)

    # Initialize the Tavily tool
    tavily_tool = TavilySearchResults(max_results=3, include_raw_content=True)

    # Define tools
    tools = [tavily_tool]

    # Bind tools to the LLM
    llm_with_tools = llm.bind_tools(tools)

    # Define the nodes
    def chatbot(state: AgentState):
        """The main LLM node that decides what to do."""
        messages = state["messages"]
        # Provide a system prompt to guide behavior
        sys_msg = SystemMessage(
            content="You are an AI research assistant. Use the Tavily search tool to find "
                    "accurate and up-to-date information to answer the user's questions. "
                    "Synthesize the search results into a clear and concise answer."
        )

        # Prepend the system message if it's the start
        if not isinstance(messages[0], SystemMessage):
            messages = [sys_msg] + messages

        response = llm_with_tools.invoke(messages)
        return {"messages": [response]}

    # Build the graph
    graph_builder = StateGraph(AgentState)

    # Add nodes
    graph_builder.add_node("chatbot", chatbot)
    graph_builder.add_node("tools", ToolNode(tools=tools))

    # Add edges
    graph_builder.add_edge(START, "chatbot")
    # The tools_condition routing logic checks if the chatbot returned a tool call.
    # If so, it routes to 'tools'. If not, it routes to END.
    graph_builder.add_conditional_edges("chatbot", tools_condition)
    # After the tools run, they should return back to the chatbot to synthesize the results.
    graph_builder.add_edge("tools", "chatbot")

    # Compile the graph
    return graph_builder.compile()

def process_query(query: str) -> str:
    """
    Takes a user query, runs it through the LangGraph agent,
    and returns the final string response.
    """
    agent = get_agent()
    initial_state = {"messages": [HumanMessage(content=query)]}

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
