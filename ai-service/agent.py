import json
import os
from typing import Annotated, Any, TypedDict, List
from datetime import datetime, timezone
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


MODULE_JSON_SCHEMA = {
    "module": "external_intelligence",
    "target": {
        "host": "example.com",
        "url": "https://example.com"
    },
    "summary": "Short security-focused summary.",
    "whois_signals": {
        "registrar": "",
        "organization": "",
        "country": "",
        "asn": "",
        "asn_org": ""
    },
    "entity_profile": {
        "organization": "",
        "industry": "",
        "headquarters_country": "",
        "description": "",
        "primary_language": "",
        "maturity_indicators": ""
    },
    "infrastructure_signals": {
        "hosting_platform": "",
        "registrar": "",
        "asn": "",
        "asn_org": "",
        "site_type": "",
        "infra_type": "",
        "waf_detected": "",
        "cdn_detected": "",
        "ssl_grade": "",
        "observed_technologies": [],
        "related_urls": [],
        "historical_urls": []
    },
    "public_security_signals": [
        "Recent breaches, advisories, leaked data mentions, or notable public references."
    ],
    "likely_risks": [
        {
            "title": "Short risk name",
            "severity": "low|medium|high|critical",
            "confidence": "low|medium|high",
            "details": "Why this matters for the target."
        }
    ],
    "attack_surface": [
        "Externally reachable assets, auth portals, APIs, admin panels, storage, or sensitive forms."
    ],
    "next_steps": [
        {
            "priority": "low|medium|high|critical",
            "action": "Concrete follow-up check a pentester should perform.",
            "details": "Why this step matters."
        }
    ],
    "sources": [
        {
            "title": "Source title",
            "url": "https://example.com",
            "reason": "Why this source matters."
        }
    ],
    "metadata": {
        "confidence": "low|medium|high",
        "generated_at": "2026-03-27T00:00:00Z"
    }
}


def is_module_mode(query: str) -> bool:
    normalized = query.lower()
    return (
        "find the latest security vulnerabilities" in normalized
        or "exposed assets" in normalized
        or "technology stack information for" in normalized
    )


def build_system_prompt(query: str) -> str:
    module_mode = is_module_mode(query)

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
            + "Return ONLY valid JSON. Do not use Markdown, code fences, prose before the JSON, or commentary after it. "
            + "The JSON must follow this exact shape and use arrays even when there is only one item: "
            + json.dumps(MODULE_JSON_SCHEMA, ensure_ascii=True)
            + " Keep entries compact, concrete, and security-focused. "
            + "If a field has no strong evidence, return an empty array or an empty string. "
            + "Do not invent sources; include only sources you can support from the search results."
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


def extract_json_object(raw: str) -> dict[str, Any] | None:
    text = raw.strip()
    if not text:
        return None

    try:
        parsed = json.loads(text)
        if isinstance(parsed, dict):
            return parsed
    except json.JSONDecodeError:
        pass

    start = text.find("{")
    if start == -1:
        return None

    depth = 0
    in_string = False
    escape = False

    for index in range(start, len(text)):
        char = text[index]

        if in_string:
            if escape:
                escape = False
            elif char == "\\":
                escape = True
            elif char == '"':
                in_string = False
            continue

        if char == '"':
            in_string = True
        elif char == "{":
            depth += 1
        elif char == "}":
            depth -= 1
            if depth == 0:
                candidate = text[start:index + 1]
                try:
                    parsed = json.loads(candidate)
                except json.JSONDecodeError:
                    return None
                return parsed if isinstance(parsed, dict) else None

    return None


def as_clean_string(value: Any) -> str:
    if value is None:
        return ""
    if isinstance(value, str):
        return value.strip()
    return str(value).strip()


def as_string_list(value: Any) -> list[str]:
    if not isinstance(value, list):
        return []

    items: list[str] = []
    for entry in value:
        if isinstance(entry, str):
            text = entry.strip()
            if text:
                items.append(text)
        elif isinstance(entry, dict):
            text = (
                as_clean_string(entry.get("value"))
                or as_clean_string(entry.get("title"))
                or as_clean_string(entry.get("detail"))
                or as_clean_string(entry.get("description"))
                or as_clean_string(entry.get("action"))
            )
            if text:
                items.append(text)
    return items


def dict_value(data: dict[str, Any], *keys: str) -> Any:
    for key in keys:
        if key in data and data[key] not in (None, "", [], {}):
            return data[key]
    return None


def normalize_whois_signals(parsed: dict[str, Any]) -> dict[str, str]:
    raw = parsed.get("whois_signals")
    if not isinstance(raw, dict):
        raw = {}

    return {
        "registrar": as_clean_string(dict_value(raw, "registrar")),
        "organization": as_clean_string(dict_value(raw, "organization", "org")),
        "country": as_clean_string(dict_value(raw, "country")),
        "asn": as_clean_string(dict_value(raw, "asn")),
        "asn_org": as_clean_string(dict_value(raw, "asn_org")),
    }


def normalize_entity_profile(parsed: dict[str, Any]) -> dict[str, str]:
    raw = parsed.get("entity_profile")
    if not isinstance(raw, dict):
        raw = {}

    return {
        "organization": as_clean_string(dict_value(raw, "organization", "company_name")),
        "industry": as_clean_string(dict_value(raw, "industry", "sector")),
        "headquarters_country": as_clean_string(dict_value(raw, "headquarters_country", "registrant_country")),
        "description": as_clean_string(dict_value(raw, "description", "notes")),
        "primary_language": as_clean_string(dict_value(raw, "primary_language")),
        "maturity_indicators": as_clean_string(dict_value(raw, "maturity_indicators", "size_estimate")),
    }


def normalize_infrastructure_signals(parsed: dict[str, Any]) -> dict[str, Any]:
    source = parsed.get("infrastructure_signals")
    if not isinstance(source, dict):
        source = parsed.get("hosting_infrastructure")
    if not isinstance(source, dict):
        source = {}

    tech_source = parsed.get("technology_stack")
    tech_lists: list[str] = []
    if isinstance(tech_source, dict):
        tech_lists.extend(as_string_list(tech_source.get("confirmed")))
        tech_lists.extend(as_string_list(tech_source.get("inferred_likely")))

    return {
        "hosting_platform": as_clean_string(dict_value(source, "hosting_platform")),
        "registrar": as_clean_string(dict_value(source, "registrar")),
        "asn": as_clean_string(dict_value(source, "asn")),
        "asn_org": as_clean_string(dict_value(source, "asn_org")),
        "site_type": as_clean_string(dict_value(source, "site_type")),
        "infra_type": as_clean_string(dict_value(source, "infra_type")),
        "waf_detected": as_clean_string(dict_value(source, "waf_detected")),
        "cdn_detected": as_clean_string(dict_value(source, "cdn_detected", "cdn_edge")),
        "ssl_grade": as_clean_string(dict_value(source, "ssl_grade", "ssl_status")),
        "observed_technologies": tech_lists + as_string_list(source.get("observed_technologies")),
        "related_urls": as_string_list(source.get("related_urls")),
        "historical_urls": as_string_list(source.get("historical_urls")),
    }


def normalize_public_security_signals(parsed: dict[str, Any]) -> list[str]:
    signals = as_string_list(parsed.get("public_security_signals"))

    threat_intelligence = parsed.get("threat_intelligence")
    if isinstance(threat_intelligence, dict):
        signals.extend(as_string_list(threat_intelligence.get("public_security_signals")))
        for vuln in threat_intelligence.get("platform_vulnerabilities", []) if isinstance(threat_intelligence.get("platform_vulnerabilities"), list) else []:
            if not isinstance(vuln, dict):
                continue
            summary = " | ".join(
                part for part in [
                    as_clean_string(vuln.get("title")),
                    as_clean_string(vuln.get("severity")).upper(),
                    as_clean_string(vuln.get("description") or vuln.get("relevance_to_target")),
                ] if part
            )
            if summary:
                signals.append(summary)

    breach_info = parsed.get("vulnerability_and_breach_intelligence")
    if isinstance(breach_info, dict):
        for key in ["known_breaches", "known_cves", "historical_security_news"]:
            text = as_clean_string(breach_info.get(key))
            if text and text.lower() != "none found in public sources for inssuria.com" and text.lower() != "none found specific to this target":
                signals.append(text)

    return list(dict.fromkeys(signal for signal in signals if signal))


def normalize_likely_risks(parsed: dict[str, Any]) -> list[dict[str, str]]:
    risks: list[dict[str, str]] = []

    for item in parsed.get("likely_risks", []) if isinstance(parsed.get("likely_risks"), list) else []:
        if not isinstance(item, dict):
            continue
        risks.append(
            {
                "title": as_clean_string(item.get("title")),
                "severity": as_clean_string(item.get("severity")).lower(),
                "confidence": as_clean_string(item.get("confidence")).lower(),
                "details": as_clean_string(item.get("details")),
            }
        )

    attack_surface_analysis = parsed.get("attack_surface_analysis")
    if isinstance(attack_surface_analysis, dict):
        for title, item in attack_surface_analysis.items():
            if not isinstance(item, dict):
                continue
            risks.append(
                {
                    "title": title.replace("_", " ").strip(),
                    "severity": as_clean_string(item.get("severity")).lower(),
                    "confidence": "medium",
                    "details": as_clean_string(item.get("detail") or item.get("recommendation")),
                }
            )

    risk_summary = parsed.get("risk_summary")
    if isinstance(risk_summary, dict):
        overall = as_clean_string(risk_summary.get("overall_risk"))
        concerns = as_string_list(risk_summary.get("key_concerns"))
        if overall or concerns:
            risks.append(
                {
                    "title": "Overall risk summary",
                    "severity": overall.lower(),
                    "confidence": "medium",
                    "details": " ".join(concerns).strip(),
                }
            )

    return [
        risk for risk in risks
        if any(risk.values())
    ]


def normalize_attack_surface(parsed: dict[str, Any]) -> list[str]:
    findings = as_string_list(parsed.get("attack_surface"))

    raw = parsed.get("attack_surface_findings")
    if isinstance(raw, dict):
        web_application = raw.get("web_application")
        if isinstance(web_application, dict):
            for label, value in web_application.items():
                text = as_clean_string(value)
                if text:
                    findings.append(f"{label.replace('_', ' ')}: {text}")

        data_sensitivity = raw.get("data_sensitivity")
        if isinstance(data_sensitivity, dict):
            details = " | ".join(
                part for part in [
                    as_clean_string(data_sensitivity.get("classification")),
                    as_clean_string(data_sensitivity.get("rationale")),
                ] if part
            )
            if details:
                findings.append(f"data sensitivity: {details}")

        for item in raw.get("potential_subdomains_or_entry_points", []) if isinstance(raw.get("potential_subdomains_or_entry_points"), list) else []:
            if not isinstance(item, dict):
                continue
            value = as_clean_string(item.get("value"))
            rationale = as_clean_string(item.get("rationale"))
            if value:
                findings.append(f"{value}: {rationale}".strip(": "))

    discovery = parsed.get("subdomain_and_asset_discovery")
    if isinstance(discovery, dict):
        note = as_clean_string(discovery.get("note"))
        if note:
            findings.append(note)

    return list(dict.fromkeys(item for item in findings if item))


def normalize_next_steps(parsed: dict[str, Any]) -> list[dict[str, str]]:
    results: list[dict[str, str]] = []

    for item in parsed.get("next_steps", []) if isinstance(parsed.get("next_steps"), list) else []:
        if not isinstance(item, dict):
            continue
        results.append(
            {
                "priority": as_clean_string(item.get("priority")).lower(),
                "action": as_clean_string(item.get("action")),
                "details": as_clean_string(item.get("details")),
            }
        )

    for item in parsed.get("next_validation_steps", []) if isinstance(parsed.get("next_validation_steps"), list) else []:
        if isinstance(item, str):
            results.append({"priority": "", "action": item.strip(), "details": ""})

    for item in parsed.get("open_questions_and_next_steps", []) if isinstance(parsed.get("open_questions_and_next_steps"), list) else []:
        if not isinstance(item, dict):
            continue
        question = as_clean_string(item.get("question"))
        action = as_clean_string(item.get("action"))
        results.append(
            {
                "priority": as_clean_string(item.get("priority")).lower(),
                "action": question or action,
                "details": action if question else "",
            }
        )

    for item in parsed.get("recommended_next_steps", []) if isinstance(parsed.get("recommended_next_steps"), list) else []:
        if not isinstance(item, dict):
            continue
        results.append(
            {
                "priority": as_clean_string(item.get("priority") or item.get("step")).lower(),
                "action": as_clean_string(item.get("action")),
                "details": as_clean_string(item.get("detail")),
            }
        )

    return [step for step in results if step["action"] or step["details"]]


def normalize_sources(parsed: dict[str, Any]) -> list[dict[str, str]]:
    results: list[dict[str, str]] = []

    for item in parsed.get("sources", []) if isinstance(parsed.get("sources"), list) else []:
        if not isinstance(item, dict):
            continue
        results.append(
            {
                "title": as_clean_string(item.get("title")),
                "url": as_clean_string(item.get("url")),
                "reason": as_clean_string(item.get("reason")),
            }
        )

    threat_intelligence = parsed.get("threat_intelligence")
    if isinstance(threat_intelligence, dict):
        for vuln in threat_intelligence.get("platform_vulnerabilities", []) if isinstance(threat_intelligence.get("platform_vulnerabilities"), list) else []:
            if not isinstance(vuln, dict):
                continue
            primary = as_clean_string(vuln.get("source"))
            if primary:
                results.append(
                    {
                        "title": as_clean_string(vuln.get("title")) or "Threat intelligence source",
                        "url": primary,
                        "reason": as_clean_string(vuln.get("relevance_to_target") or vuln.get("description")),
                    }
                )
            for ref in vuln.get("references", []) if isinstance(vuln.get("references"), list) else []:
                if isinstance(ref, str) and ref.strip():
                    results.append(
                        {
                            "title": as_clean_string(vuln.get("title")) or "Threat intelligence reference",
                            "url": ref.strip(),
                            "reason": "Supporting reference",
                        }
                    )

    return [
        source for source in results
        if source["url"]
    ]


def normalize_module_response(query: str, raw: str) -> str:
    parsed = extract_json_object(raw) or {}
    target = parsed.get("target")
    target_host = ""
    target_url = ""
    if isinstance(target, dict):
        target_host = as_clean_string(target.get("host"))
        target_url = as_clean_string(target.get("url"))
    else:
        target_url = as_clean_string(target)

    whois_signals = normalize_whois_signals(parsed)
    entity_profile = normalize_entity_profile(parsed)
    infrastructure_signals = normalize_infrastructure_signals(parsed)

    if not target_host:
        target_host = (
            infrastructure_signals.get("host", "")
            or whois_signals.get("organization", "")
        )

    summary = as_clean_string(parsed.get("summary"))
    if not summary:
        risk_summary = parsed.get("risk_summary")
        if isinstance(risk_summary, dict):
            summary = " ".join(as_string_list(risk_summary.get("key_concerns"))[:2]).strip()
    if not summary:
        summary = as_clean_string(dict_value(entity_profile, "description"))

    metadata = parsed.get("metadata")
    if not isinstance(metadata, dict):
        metadata = {}

    normalized: dict[str, Any] = {
        "module": "external_intelligence",
        "target": {
            "host": target_host,
            "url": target_url or query,
        },
        "summary": summary,
        "whois_signals": whois_signals,
        "entity_profile": entity_profile,
        "infrastructure_signals": infrastructure_signals,
        "public_security_signals": normalize_public_security_signals(parsed),
        "likely_risks": normalize_likely_risks(parsed),
        "attack_surface": normalize_attack_surface(parsed),
        "next_steps": normalize_next_steps(parsed),
        "sources": normalize_sources(parsed),
        "metadata": {
            "confidence": as_clean_string(dict_value(metadata, "confidence", "overall_assessment")),
            "generated_at": as_clean_string(dict_value(metadata, "generated_at", "timestamp")) or datetime.now(timezone.utc).isoformat().replace("+00:00", "Z"),
        },
    }

    return json.dumps(normalized, ensure_ascii=True, indent=2)

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
        response_text = extract_text_content(last_message.content)
        if is_module_mode(query):
            return normalize_module_response(query, response_text)
        return response_text

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
