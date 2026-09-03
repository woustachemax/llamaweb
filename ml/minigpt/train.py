import json
import math
import os
import time

import numpy as np
import torch

from config import ModelConfig, TrainConfig, OUT_DIR, DEVICE, BASE_ENCODING
from minigpt.model import MiniGPT
from minigpt.tokenizer import VoiceTokenizer


def _read_corpus(path):
    if not os.path.exists(path):
        raise FileNotFoundError(f"corpus not found: {path}")
    with open(path, "r", encoding="utf-8", errors="ignore") as f:
        text = f.read()
    if not text.strip():
        raise ValueError("corpus is empty; chat with the assistant first to collect your voice")
    return text


def _fit_config(n_tokens):
    m = ModelConfig()
    if n_tokens < 2000:
        m.block_size, m.n_layer, m.n_head, m.n_embd = 32, 3, 3, 96
    elif n_tokens < 20000:
        m.block_size, m.n_layer, m.n_head, m.n_embd = 64, 4, 4, 128
    else:
        m.block_size, m.n_layer, m.n_head, m.n_embd = 128, 4, 4, 192
    return m


def _get_batch(data, block_size, batch_size, device):
    ix = torch.randint(len(data) - block_size - 1, (batch_size,))
    x = torch.stack([torch.from_numpy(data[i : i + block_size].astype(np.int64)) for i in ix])
    y = torch.stack([torch.from_numpy(data[i + 1 : i + 1 + block_size].astype(np.int64)) for i in ix])
    return x.to(device), y.to(device)


@torch.no_grad()
def _estimate_loss(model, splits, tc, device):
    out = {}
    model.eval()
    for name, data in splits.items():
        if data is None or len(data) <= model.cfg.block_size + 1:
            out[name] = float("nan")
            continue
        losses = torch.zeros(tc.eval_iters)
        for k in range(tc.eval_iters):
            x, y = _get_batch(data, model.cfg.block_size, tc.batch_size, device)
            _, loss = model(x, y)
            losses[k] = loss.item()
        out[name] = losses.mean().item()
    model.train()
    return out


def _lr(step, tc):
    if step < tc.warmup_steps:
        return tc.learning_rate * (step + 1) / max(tc.warmup_steps, 1)
    progress = (step - tc.warmup_steps) / max(tc.steps - tc.warmup_steps, 1)
    return 0.1 * tc.learning_rate + 0.5 * (1 + math.cos(math.pi * min(progress, 1.0))) * (
        tc.learning_rate - 0.1 * tc.learning_rate
    )


def train(corpus_path, steps=None, out_dir=OUT_DIR):
    t0 = time.time()
    device = torch.device(DEVICE if torch.cuda.is_available() or DEVICE == "cpu" else "cpu")
    text = _read_corpus(corpus_path)

    tok = VoiceTokenizer(BASE_ENCODING)
    tok.build(text, max_vocab=ModelConfig.vocab_size)
    ids = np.array(tok.encode(text), dtype=np.int64)
    if len(ids) < 64:
        raise ValueError(f"corpus too small after tokenization ({len(ids)} tokens)")

    mc = _fit_config(len(ids))
    mc.vocab_size = tok.vocab_size

    tc = TrainConfig()
    if steps:
        tc.steps = max(20, int(steps))
    if len(ids) < 4000:
        tc.batch_size = 12

    torch.manual_seed(tc.seed)
    n_val = max(mc.block_size + 2, int(len(ids) * tc.val_split))
    if n_val >= len(ids) - mc.block_size - 2:
        train_data, val_data = ids, None
    else:
        train_data, val_data = ids[:-n_val], ids[-n_val:]

    model = MiniGPT(mc).to(device)
    optimizer = torch.optim.AdamW(
        model.parameters(), lr=tc.learning_rate, weight_decay=tc.weight_decay, betas=(0.9, 0.99)
    )

    splits = {"train": train_data, "val": val_data}
    best_val = float("inf")
    last_val = float("nan")
    model.train()
    for step in range(tc.steps):
        for g in optimizer.param_groups:
            g["lr"] = _lr(step, tc)
        x, y = _get_batch(train_data, mc.block_size, tc.batch_size, device)
        _, loss = model(x, y)
        optimizer.zero_grad(set_to_none=True)
        loss.backward()
        torch.nn.utils.clip_grad_norm_(model.parameters(), tc.grad_clip)
        optimizer.step()

        if step % tc.eval_interval == 0 or step == tc.steps - 1:
            est = _estimate_loss(model, splits, tc, device)
            last_val = est["val"] if not math.isnan(est["val"]) else est["train"]
            best_val = min(best_val, last_val)
            print(f"step {step:4d} | train {est['train']:.4f} | val {last_val:.4f} | lr {optimizer.param_groups[0]['lr']:.2e}")

    os.makedirs(out_dir, exist_ok=True)
    torch.save(
        {"model": model.state_dict(), "config": mc.to_dict()},
        os.path.join(out_dir, "ckpt.pt"),
    )
    tok.save(os.path.join(out_dir, "vocab.json"))
    meta = {
        "trained_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        "steps": tc.steps,
        "val_loss": None if math.isnan(last_val) else round(float(last_val), 4),
        "vocab_size": tok.vocab_size,
        "params": model.num_params(),
        "corpus_chars": len(text),
        "corpus_tokens": int(len(ids)),
        "config": mc.to_dict(),
        "seconds": round(time.time() - t0, 2),
    }
    with open(os.path.join(out_dir, "meta.json"), "w") as f:
        json.dump(meta, f, indent=2)
    return meta


if __name__ == "__main__":
    import sys

    from config import DEFAULT_CORPUS

    path = sys.argv[1] if len(sys.argv) > 1 else DEFAULT_CORPUS
    print(json.dumps(train(path), indent=2))
