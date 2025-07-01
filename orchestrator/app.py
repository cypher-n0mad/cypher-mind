# orchestrator/app.py
# FastAPI server tailored for Ollama local LLM inference
# Requirements:
#   pip install fastapi uvicorn python-dotenv
# Usage:
#   OLLAMA_MODEL=llama3:8b-q4_K_M \
#   uvicorn orchestrator.app:app --uds /run/ai.sock --reload

import os
import shlex
import subprocess
from fastapi import FastAPI, HTTPException, Request
from fastapi.responses import StreamingResponse
from dotenv import load_dotenv

# Load environment variables from .env if present
load_dotenv()

app = FastAPI(
    title="AI Orchestrator (Ollama)",
    description="Routes chat requests to a local Ollama LLM",
)

# Ollama model identifier
OLLAMA_CMD = os.getenv("OLLAMA_CMD", "ollama")  # ollama binary
OLLAMA_MODEL = os.getenv("OLLAMA_MODEL", "gemma3:1b")

@app.post("/v1/chat/completions")
async def chat_completions(request: Request):
    """
    Expect JSON payload:
      {"messages": [{"role": "user", "content": "..."}, ...]}
    Uses the last user message as the prompt.
    Streams back plain-text token output.
    """
    try:
        payload = await request.json()
    except Exception:
        raise HTTPException(status_code=400, detail="Invalid JSON payload")

    messages = payload.get("messages")
    if not isinstance(messages, list) or not messages:
        raise HTTPException(status_code=400, detail="No messages provided or invalid format")

    # Only consider the last user message
    last = messages[-1]
    role = last.get("role")
    if role != "user":
        raise HTTPException(status_code=400, detail="Last message must have role 'user'")

    prompt = last.get("content", "").strip()
    if not prompt:
        raise HTTPException(status_code=400, detail="Empty prompt")

    cmd = [OLLAMA_CMD, "run", OLLAMA_MODEL, prompt]

    try:
        proc = subprocess.Popen(
            cmd,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            bufsize=1,
        )
    except FileNotFoundError:
        raise HTTPException(status_code=500, detail=f"Ollama command not found: {OLLAMA_CMD}")

    def generate():
        # Stream each line from stdout
        for line in proc.stdout:
            yield line
        proc.stdout.close()
        return_code = proc.wait()
        if return_code != 0:
            err = proc.stderr.read() or ""
            # Stop iteration and surface HTTP error
            raise HTTPException(status_code=500, detail=f"Ollama failed ({return_code}): {err}")

    return StreamingResponse(generate(), media_type="text/plain")
