# ReconX AI Service

This is a Python microservice that uses FastAPI, LangGraph, OpenAI, and Tavily to provide AI research capabilities.

## Requirements

- Python 3.9+
- An [OpenAI API Key](https://platform.openai.com/api-keys)
- A [Tavily API Key](https://tavily.com/)

## Installation

1. Navigate to this directory:
   ```bash
   cd ai-service
   ```

2. Create a virtual environment (optional but recommended):
   ```bash
   python -m venv venv
   source venv/bin/activate  # On Windows, use `venv\Scripts\activate`
   ```

3. Install the dependencies:
   ```bash
   pip install -r requirements.txt
   ```

## Configuration

1. Create a `.env` file in the `ai-service` directory (or export the variables in your shell):
   ```bash
   touch .env
   ```

2. Add your API keys to the `.env` file:
   ```env
   OPENAI_API_KEY=your_openai_api_key_here
   TAVILY_API_KEY=your_tavily_api_key_here
   ```

## Running the Service

Start the FastAPI server using Uvicorn:

```bash
uvicorn main:app --reload --host 0.0.0.0 --port 8000
```

The service will be available at `http://localhost:8000`.

## API Endpoints

### 1. Health Check
```bash
curl http://localhost:8000/health
```

### 2. Research Query
Send a POST request to `/research` to ask the AI agent a question.

```bash
curl -X POST http://localhost:8000/research \
     -H "Content-Type: application/json" \
     -d '{"query": "What are the latest features in Go 1.22?"}'
```

## Testing Locally (CLI)

You can also run the agent directly from the command line without starting the server:

```bash
python agent.py "What are the latest features in Go 1.22?"
```
