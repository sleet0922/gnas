import os
import sys

import torch
from fastapi import FastAPI, HTTPException
from modelscope import snapshot_download
from pydantic import BaseModel

from qwen3_vl_embedding import Qwen3VLEmbedder


MODEL_ID = "Qwen/Qwen3-VL-Embedding-2B"
EMBEDDING_DIMENSION = 2048

app = FastAPI()
embedder = None
device = "cpu"


def _system_memory_bytes():
    try:
        page_size = os.sysconf("SC_PAGE_SIZE")
        page_count = os.sysconf("SC_PHYS_PAGES")
        return page_size * page_count
    except (AttributeError, OSError, ValueError):
        return 0


_cpu_count = os.cpu_count() or 1
_total_memory = _system_memory_bytes()
_memory_concurrency = 1
if _total_memory > (4 << 30):
    _memory_concurrency = max(1, int((_total_memory - (4 << 30)) // (2 << 30)))
_request_concurrency = max(1, min(_memory_concurrency, max(1, _cpu_count // 4)))

try:
    _embed_threads = int(os.environ.get("GNAS_EMBED_THREADS", "0"))
except ValueError:
    _embed_threads = 0
if _embed_threads <= 0:
    _embed_threads = max(1, _cpu_count // _request_concurrency)
torch.set_num_threads(_embed_threads)
torch.set_num_interop_threads(1)


class EmbedRequest(BaseModel):
    image_path: str = None
    text: str = None


@app.on_event("startup")
def load_model():
    global embedder, device
    try:
        current_dir = os.path.dirname(os.path.abspath(__file__))
        default_data_dir = os.path.dirname(current_dir)
        default_cache_dir = os.path.join(default_data_dir, "modelscope_cache")
        cache_dir = os.environ.get("MODELSCOPE_CACHE", default_cache_dir)
        os.makedirs(cache_dir, exist_ok=True)

        print(f"Loading model {MODEL_ID} from ModelScope...")
        local_dir = snapshot_download(MODEL_ID, cache_dir=cache_dir)
        device = "cuda" if torch.cuda.is_available() else "cpu"
        # Keep the published BF16 weights to avoid doubling the 2B model in RAM.
        embedder = Qwen3VLEmbedder(
            model_name_or_path=local_dir,
            torch_dtype=torch.bfloat16,
        )
        print(
            f"Loaded {MODEL_ID} on {device}; dimension={EMBEDDING_DIMENSION}, "
            f"concurrency={_request_concurrency}, threads={_embed_threads}"
        )
    except Exception as exc:
        print(f"Error loading model: {exc}", file=sys.stderr)
        raise


@app.get("/health")
def health():
    if embedder is None:
        raise HTTPException(status_code=503, detail="Model not loaded yet")
    return {
        "status": "ok",
        "device": device,
        "model": MODEL_ID,
        "dimension": EMBEDDING_DIMENSION,
        "request_concurrency": _request_concurrency,
        "threads_per_request": _embed_threads,
    }


@app.post("/embed")
def embed(req: EmbedRequest):
    if embedder is None:
        raise HTTPException(status_code=503, detail="Model not initialized")
    if not req.image_path and not req.text:
        raise HTTPException(status_code=400, detail="Must provide either image_path or text")
    if req.image_path and not os.path.exists(req.image_path):
        raise HTTPException(status_code=400, detail=f"Image not found: {req.image_path}")

    try:
        item = {}
        if req.text:
            item["text"] = req.text
        if req.image_path:
            item["image"] = req.image_path
        if req.image_path and req.text:
            item["instruction"] = "Represent the multimodal input for image retrieval."
        elif req.image_path:
            item["instruction"] = "Represent the image for retrieval."
        else:
            item["instruction"] = "Find an image that matches the user's query."

        with torch.inference_mode():
            vector = embedder.process([item])[0].float().cpu().tolist()
        return {"embedding": vector}
    except HTTPException:
        raise
    except Exception as exc:
        import traceback

        traceback.print_exc()
        raise HTTPException(status_code=500, detail=str(exc)) from exc


if __name__ == "__main__":
    import uvicorn

    uvicorn.run(app, host="127.0.0.1", port=8000)
