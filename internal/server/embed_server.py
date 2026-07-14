import argparse
import sys
import os
import torch
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
from transformers import AutoProcessor, AutoModel
from PIL import Image
from modelscope import snapshot_download

app = FastAPI()

# Global model and processor
model = None
processor = None
device = "cpu"

class EmbedRequest(BaseModel):
    image_path: str = None
    text: str = None

def last_token_pool(last_hidden_states: torch.Tensor, attention_mask: torch.Tensor) -> torch.Tensor:
    left_padding = attention_mask[:, -1].sum() == attention_mask.shape[0]
    if left_padding:
        return last_hidden_states[:, -1]
    sequence_lengths = attention_mask.sum(dim=1) - 1
    batch_size = last_hidden_states.shape[0]
    return last_hidden_states[torch.arange(batch_size, device=last_hidden_states.device), sequence_lengths]

@app.on_event("startup")
def load_model():
    global model, processor, device
    try:
        model_id = "Qwen/Qwen3-VL-Embedding-2B"
        print(f"Loading model {model_id} from ModelScope...")
        
        # Ensure model is cached
        current_dir = os.path.dirname(os.path.abspath(__file__))
        default_data_dir = os.path.dirname(current_dir) # qwen3_vl_ov's parent
        default_cache_dir = os.path.join(default_data_dir, "modelscope_cache")
        cache_dir = os.environ.get("MODELSCOPE_CACHE", default_cache_dir)
        os.makedirs(cache_dir, exist_ok=True)
        local_dir = snapshot_download(model_id, cache_dir=cache_dir)
        
        # Load processor & model
        processor = AutoProcessor.from_pretrained(local_dir, trust_remote_code=True)
        model = AutoModel.from_pretrained(local_dir, trust_remote_code=True).eval()
        
        if torch.cuda.is_available():
            device = "cuda"
            model = model.to(device)
            print("Loaded model to GPU (CUDA)")
        else:
            device = "cpu"
            print("Loaded model to CPU")
            
    except Exception as e:
        print(f"Error loading model: {e}", file=sys.stderr)
        # We don't exit to allow FastAPI to start and report health error or reload
        raise e

@app.get("/health")
def health():
    if model is None or processor is None:
        raise HTTPException(status_code=503, detail="Model not loaded yet")
    return {"status": "ok", "device": device}

@app.post("/embed")
def embed(req: EmbedRequest):
    if model is None or processor is None:
        raise HTTPException(status_code=503, detail="Model not initialized")
        
    try:
        content = []
        if req.image_path:
            if not os.path.exists(req.image_path):
                raise HTTPException(status_code=400, detail=f"Image not found: {req.image_path}")
            content.append({"type": "image", "image": req.image_path})
            
        if req.text:
            content.append({"type": "text", "text": req.text})
            
        if not content:
            raise HTTPException(status_code=400, detail="Must provide either image_path or text")

        from qwen_vl_utils import process_vision_info
        
        conversation = [
            {"role": "system", "content": [{"type": "text", "text": "Represent the input."}]},
            {"role": "user", "content": content}
        ]
        
        prompt = processor.apply_chat_template(conversation, tokenize=False, add_generation_prompt=True)
        image_inputs, video_inputs = process_vision_info(conversation)
        
        inputs = processor(
            text=[prompt], 
            images=image_inputs, 
            videos=video_inputs, 
            padding=True, 
            return_tensors="pt"
        )
        
        # Move to GPU if available
        inputs = {k: v.to(device) if isinstance(v, torch.Tensor) else v for k, v in inputs.items()}
        
        with torch.no_grad():
            outputs = model(**inputs)
            
        emb = last_token_pool(outputs.last_hidden_state, inputs["attention_mask"])[0]
        emb = emb / torch.norm(emb, p=2, dim=-1, keepdim=True)
        
        return {"embedding": emb.tolist()}
        
    except Exception as e:
        import traceback
        traceback.print_exc()
        raise HTTPException(status_code=500, detail=str(e))

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="127.0.0.1", port=8000)
