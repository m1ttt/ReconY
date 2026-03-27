from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
import os
from agent import process_query

app = FastAPI(title="ReconX AI Service", description="AI Research agent using LangGraph, OpenAI, and Tavily")

from typing import Optional

class QueryRequest(BaseModel):
    query: str
    openai_api_key: Optional[str] = None
    tavily_api_key: Optional[str] = None

class QueryResponse(BaseModel):
    result: str

@app.post("/research", response_model=QueryResponse)
def research(request: QueryRequest):
    if not request.query:
        raise HTTPException(status_code=400, detail="Query cannot be empty")

    try:
        # Override environment variables if keys are provided in the request
        if request.openai_api_key:
            os.environ["OPENAI_API_KEY"] = request.openai_api_key
        if request.tavily_api_key:
            os.environ["TAVILY_API_KEY"] = request.tavily_api_key

        # Call the LangGraph agent
        result = process_query(request.query)
        return QueryResponse(result=result)
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Error processing query: {str(e)}")

@app.get("/health")
async def health_check():
    return {"status": "ok"}
