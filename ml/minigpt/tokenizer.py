import json
import os
from collections import Counter

import tiktoken

EOT = "<|endoftext|>"


class VoiceTokenizer:
    def __init__(self, base_encoding="gpt2"):
        self.base = tiktoken.get_encoding(base_encoding)
        self.base_encoding = base_encoding
        self.eot_base = self.base.encode(EOT, allowed_special={EOT})[0]
        self.itos_base = [self.eot_base]
        self.stoi = {self.eot_base: 0}

    @property
    def vocab_size(self):
        return len(self.itos_base)

    @property
    def eot_id(self):
        return self.stoi[self.eot_base]

    def build(self, text, max_vocab=8192):
        ids = self.base.encode(text, allowed_special={EOT})
        counts = Counter(ids)
        counts.pop(self.eot_base, None)
        kept = [tok for tok, _ in counts.most_common(max_vocab - 1)]
        self.itos_base = [self.eot_base] + kept
        self.stoi = {tok: i for i, tok in enumerate(self.itos_base)}
        return self.vocab_size

    def encode(self, text, add_eot=False):
        ids = self.base.encode(text, allowed_special={EOT})
        out = [self.stoi[t] for t in ids if t in self.stoi]
        if add_eot:
            out.append(self.eot_id)
        return out

    def decode(self, ids):
        base_ids = [self.itos_base[i] for i in ids if 0 <= i < len(self.itos_base)]
        base_ids = [b for b in base_ids if b != self.eot_base]
        if not base_ids:
            return ""
        return self.base.decode(base_ids)

    def save(self, path):
        os.makedirs(os.path.dirname(path), exist_ok=True)
        with open(path, "w") as f:
            json.dump(
                {"base_encoding": self.base_encoding, "itos_base": self.itos_base},
                f,
            )

    @classmethod
    def load(cls, path):
        with open(path) as f:
            data = json.load(f)
        tok = cls(data.get("base_encoding", "gpt2"))
        tok.itos_base = [int(x) for x in data["itos_base"]]
        tok.stoi = {tok_id: i for i, tok_id in enumerate(tok.itos_base)}
        return tok
