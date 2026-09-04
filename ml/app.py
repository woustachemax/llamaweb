import threading

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel

from config import DEFAULT_CORPUS, out_dir_for
from minigpt import sample, train as trainer

app = FastAPI(title="minigpt voice service")
_train_lock = threading.Lock()


def _resolve(user):
    try:
        return out_dir_for(user)
    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e))


class GenerateRequest(BaseModel):
    user: str
    prompt: str = ""
    max_tokens: int = 200
    temperature: float = 0.9
    top_k: int = 40


class RewriteRequest(BaseModel):
    user: str
    draft: str
    context: str = ""
    temperature: float = 0.8
    max_tokens: int = 400


class ScoreRequest(BaseModel):
    user: str
    candidates: list[str]
    context: str = ""


class TrainRequest(BaseModel):
    user: str
    corpus_path: str | None = None
    steps: int | None = None


@app.get("/health")
def health():
    return {"status": "ok"}


@app.get("/status")
def status(user: str):
    return sample.status(_resolve(user))


@app.post("/generate")
def generate(req: GenerateRequest):
    try:
        return sample.generate(
            req.prompt,
            max_tokens=req.max_tokens,
            temperature=req.temperature,
            top_k=req.top_k,
            out_dir=_resolve(req.user),
        )
    except RuntimeError as e:
        raise HTTPException(status_code=409, detail=str(e))


@app.post("/rewrite")
def rewrite(req: RewriteRequest):
    try:
        return sample.rewrite(
            req.draft,
            context=req.context,
            temperature=req.temperature,
            max_tokens=req.max_tokens,
            out_dir=_resolve(req.user),
        )
    except RuntimeError as e:
        raise HTTPException(status_code=409, detail=str(e))


@app.post("/score")
def score(req: ScoreRequest):
    if not req.candidates:
        raise HTTPException(status_code=400, detail="no candidates provided")
    try:
        return sample.score(req.candidates, context=req.context, out_dir=_resolve(req.user))
    except RuntimeError as e:
        raise HTTPException(status_code=409, detail=str(e))


@app.post("/train")
def train(req: TrainRequest):
    out_dir = _resolve(req.user)
    if not _train_lock.acquire(blocking=False):
        raise HTTPException(status_code=429, detail="training already in progress")
    try:
        corpus = req.corpus_path or DEFAULT_CORPUS
        meta = trainer.train(corpus, steps=req.steps, out_dir=out_dir)
        sample.load(out_dir, force=True)
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
