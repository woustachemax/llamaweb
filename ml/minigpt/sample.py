import json
import os
import threading

import torch

from config import ModelConfig, OUT_DIR, DEVICE
from minigpt.model import MiniGPT
from minigpt.tokenizer import VoiceTokenizer

_lock = threading.Lock()
_states = {}


def _ckpt_paths(out_dir):
    return (
        os.path.join(out_dir, "ckpt.pt"),
        os.path.join(out_dir, "vocab.json"),
        os.path.join(out_dir, "meta.json"),
    )


def is_trained(out_dir):
    ckpt, vocab, _ = _ckpt_paths(out_dir)
    return os.path.exists(ckpt) and os.path.exists(vocab)


def load(out_dir, force=False):
    ckpt_path, vocab_path, meta_path = _ckpt_paths(out_dir)
    if not is_trained(out_dir):
        raise RuntimeError("voice model is not trained yet")
    mtime = os.path.getmtime(ckpt_path)
    with _lock:
        cached = _states.get(out_dir)
        if not force and cached is not None and cached["mtime"] == mtime:
            return cached["model"], cached["tok"], cached["meta"]
        device = torch.device(DEVICE if DEVICE == "cpu" or torch.cuda.is_available() else "cpu")
        blob = torch.load(ckpt_path, map_location=device)
        mc = ModelConfig(**blob["config"])
        model = MiniGPT(mc).to(device)
        model.load_state_dict(blob["model"])
        model.eval()
        tok = VoiceTokenizer.load(vocab_path)
        meta = {}
        if os.path.exists(meta_path):
            with open(meta_path) as f:
                meta = json.load(f)
        _states[out_dir] = {"model": model, "tok": tok, "meta": meta, "mtime": mtime}
        return model, tok, meta


def status(out_dir):
    if not is_trained(out_dir):
        return {"trained": False, "notes": ["train the voice model from the app or POST /train"]}
    _, tok, meta = load(out_dir)
    out = {"trained": True, "vocab_size": tok.vocab_size}
    out.update(meta)
    return out


@torch.no_grad()
def generate(prompt, max_tokens=200, temperature=0.9, top_k=40, out_dir=OUT_DIR):
    model, tok, _ = load(out_dir)
    device = next(model.parameters()).device
    ids = tok.encode(prompt) if prompt else [tok.eot_id]
    ids = ids[-model.cfg.block_size :] or [tok.eot_id]
    idx = torch.tensor([ids], dtype=torch.long, device=device)
    out = model.generate(idx, max_tokens, temperature=temperature, top_k=top_k, eot_id=tok.eot_id)
    completion_ids = out[0].tolist()[len(ids):]
    return {
        "text": tok.decode(out[0].tolist()),
        "completion": tok.decode(completion_ids),
    }


@torch.no_grad()
def rewrite(draft, context="", temperature=0.8, max_tokens=400, out_dir=OUT_DIR):
    model, tok, _ = load(out_dir)
    device = next(model.parameters()).device
    seed_text = (context + "\n" + draft).strip() + "\n"
    ids = tok.encode(seed_text)
    ids = ids[-model.cfg.block_size :] or [tok.eot_id]
    idx = torch.tensor([ids], dtype=torch.long, device=device)
    out = model.generate(idx, max_tokens, temperature=temperature, top_k=40, eot_id=tok.eot_id)
    generated = tok.decode(out[0].tolist()[len(ids):]).strip()
    return {"text": generated or draft}


@torch.no_grad()
def score(candidates, context="", out_dir=OUT_DIR):
    model, tok, _ = load(out_dir)
    device = next(model.parameters()).device
    ctx_ids = tok.encode(context + "\n") if context else []
    ranked = []
    for cand in candidates:
        ids = ctx_ids + tok.encode(cand)
        if len(ids) < 2:
            ranked.append({"text": cand, "score": -1e9, "perplexity": float("inf")})
            continue
        idx = torch.tensor([ids], dtype=torch.long, device=device)
        avg_lp, _ = model.sequence_logprob(idx)
        ranked.append(
            {
                "text": cand,
                "score": round(float(avg_lp), 5),
                "perplexity": round(float(torch.exp(torch.tensor(-avg_lp)).item()), 3),
            }
        )
    ranked.sort(key=lambda r: r["score"], reverse=True)
    best = ranked[0]["text"] if ranked else ""
    return {"ranked": ranked, "best": best}
