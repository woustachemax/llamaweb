import os
import re
from dataclasses import dataclass, asdict


@dataclass
class ModelConfig:
    vocab_size: int = 8192
    block_size: int = 128
    n_layer: int = 4
    n_head: int = 4
    n_embd: int = 192
    dropout: float = 0.1
    bias: bool = True

    def to_dict(self):
        return asdict(self)


@dataclass
class TrainConfig:
    steps: int = 600
    batch_size: int = 24
    learning_rate: float = 3e-4
    weight_decay: float = 0.1
    warmup_steps: int = 40
    grad_clip: float = 1.0
    eval_interval: int = 100
    eval_iters: int = 40
    val_split: float = 0.1
    seed: int = 1337


ROOT = os.path.dirname(os.path.abspath(__file__))
OUT_DIR = os.environ.get("MINIGPT_OUT_DIR", os.path.join(ROOT, "checkpoints"))
DATA_DIR = os.path.join(ROOT, "data")
DEFAULT_CORPUS = os.environ.get("MINIGPT_CORPUS", os.path.join(DATA_DIR, "user_corpus.txt"))
DEVICE = os.environ.get("MINIGPT_DEVICE", "cpu")
BASE_ENCODING = os.environ.get("MINIGPT_BASE_ENCODING", "gpt2")

_USER_RE = re.compile(r"[^a-zA-Z0-9_-]")


def out_dir_for(user):
    slug = _USER_RE.sub("", user or "")
    if not slug:
        raise ValueError("missing or invalid user id")
    return os.path.join(OUT_DIR, slug)
