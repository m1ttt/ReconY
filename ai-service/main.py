from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
import os
from agent import process_query

app = FastAPI(title="ReconX AI Service", description="AI Research agent using LangGraph, OpenAI, and Tavily")

class QueryRequest(BaseModel):
    query: str

class QueryResponse(BaseModel):
    result: str

@app.post("/research", response_model=QueryResponse)
def research(request: QueryRequest):
    if not request.query:
        raise HTTPException(status_code=400, detail="Query cannot be empty")

    try:
        # Call the LangGraph agent
        result = process_query(request.query)
        return QueryResponse(result=result)
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Error processing query: {str(e)}")

@app.get("/health")
async def health_check():
    return {"status": "ok"}
