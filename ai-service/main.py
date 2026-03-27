from fastapi import FastAPI, HTTPException
from fastapi.responses import StreamingResponse
from pydantic import BaseModel
import os
import json
from agent import process_query, stream_query_events

app = FastAPI(title="ReconX AI Service", description="AI Research agent using LangGraph, OpenAI, and Tavily")

from typing import Optional


class ConversationMessage(BaseModel):
    role: str
    content: str

class QueryRequest(BaseModel):
    query: str
    messages: list[ConversationMessage] = []
    openai_api_key: Optional[str] = None
    openai_base_url: Optional[str] = None
    openai_model: Optional[str] = None
    tavily_api_key: Optional[str] = None

class QueryResponse(BaseModel):
    result: str


def apply_request_env(request: QueryRequest):
    if request.openai_api_key:
        os.environ["OPENAI_API_KEY"] = request.openai_api_key
    if request.openai_base_url:
        os.environ["OPENAI_BASE_URL"] = request.openai_base_url
    if request.tavily_api_key:
        os.environ["TAVILY_API_KEY"] = request.tavily_api_key

@app.post("/research", response_model=QueryResponse)
def research(request: QueryRequest):
    if not request.query:
        raise HTTPException(status_code=400, detail="Query cannot be empty")

    try:
        apply_request_env(request)

        # Call the LangGraph agent
        result = process_query(
            request.query,
            request.openai_model,
            [message.model_dump() for message in request.messages],
        )
        return QueryResponse(result=result)
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Error processing query: {str(e)}")


@app.post("/research/stream")
async def research_stream(request: QueryRequest):
    if not request.query:
        raise HTTPException(status_code=400, detail="Query cannot be empty")

    async def event_stream():
        try:
            apply_request_env(request)
            async for event in stream_query_events(
                request.query,
                request.openai_model,
                [message.model_dump() for message in request.messages],
            ):
                yield f"data: {json.dumps(event)}\n\n"
        except Exception as e:
            yield f"data: {json.dumps({'type': 'error', 'error': f'Error processing query: {str(e)}'})}\n\n"

    return StreamingResponse(
        event_stream(),
        media_type="text/event-stream",
        headers={
            "Cache-Control": "no-cache",
            "Connection": "keep-alive",
            "X-Accel-Buffering": "no",
        },
    )

@app.get("/health")
async def health_check():
    return {"status": "ok"}
