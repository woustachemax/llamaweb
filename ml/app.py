import threading

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel

from config import DEFAULT_CORPUS, OUT_DIR
from minigpt import sample, train as trainer

app = FastAPI(title="minigpt voice service")
_train_lock = threading.Lock()


class GenerateRequest(BaseModel):
    prompt: str = ""
    max_tokens: int = 200
    temperature: float = 0.9
    top_k: int = 40


class RewriteRequest(BaseModel):
    draft: str
    context: str = ""
    temperature: float = 0.8
    max_tokens: int = 400


class ScoreRequest(BaseModel):
    candidates: list[str]
    context: str = ""


class TrainRequest(BaseModel):
    corpus_path: str | None = None
    steps: int | None = None


@app.get("/health")
def health():
    return {"status": "ok", "trained": sample.is_trained(OUT_DIR)}


@app.get("/status")
def status():
    return sample.status(OUT_DIR)


@app.post("/generate")
def generate(req: GenerateRequest):
    try:
        return sample.generate(
            req.prompt, max_tokens=req.max_tokens, temperature=req.temperature, top_k=req.top_k
        )
    except RuntimeError as e:
        raise HTTPException(status_code=409, detail=str(e))


@app.post("/rewrite")
def rewrite(req: RewriteRequest):
    try:
        return sample.rewrite(
            req.draft, context=req.context, temperature=req.temperature, max_tokens=req.max_tokens
        )
    except RuntimeError as e:
        raise HTTPException(status_code=409, detail=str(e))


@app.post("/score")
def score(req: ScoreRequest):
    if not req.candidates:
        raise HTTPException(status_code=400, detail="no candidates provided")
    try:
        return sample.score(req.candidates, context=req.context)
    except RuntimeError as e:
        raise HTTPException(status_code=409, detail=str(e))


@app.post("/train")
def train(req: TrainRequest):
    if not _train_lock.acquire(blocking=False):
        raise HTTPException(status_code=429, detail="training already in progress")
    try:
        corpus = req.corpus_path or DEFAULT_CORPUS
        meta = trainer.train(corpus, steps=req.steps, out_dir=OUT_DIR)
        sample.load(OUT_DIR, force=True)
        return {
            "status": "trained",
            "steps": meta["steps"],
            "val_loss": meta["val_loss"],
            "vocab_size": meta["vocab_size"],
            "params": meta["params"],
            "seconds": meta["seconds"],
        }
    except (FileNotFoundError, ValueError) as e:
        raise HTTPException(status_code=400, detail=str(e))
    finally:
        _train_lock.release()
