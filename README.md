# llamaweb

Ollama, but in a browser. A Go agent runs the model; a PyTorch miniGPT trained on
your own messages picks the reply that sounds most like you.

## Quickstart

Requires Go 1.27+, Python 3.10+, Node 18+, and [Ollama](https://ollama.com) running:

```bash
ollama pull llama3.2
make setup      
./run.sh       
```

Open http://localhost:5173, create an account, chat a few times, then hit
**train on my messages** in the right panel to build your voice model.

Run processes separately with `make ml`, `make backend`, `make frontend`.

## Layout

| Path        | What                                                                    |
|-------------|------------------------------------------------------------------------|
| `backend/`  | Go: accounts, sessions, SSE chat, agent loop, Ollama-compatible API   |
| `ml/`       | FastAPI + nanoGPT-style transformer: `/generate` `/score` `/rewrite` `/train` `/status` |
| `frontend/` | React chat UI                                                          |

## Voice layer

Your messages are logged to `ml/data/user_corpus.txt`. Training fine-tunes the
miniGPT on next-token prediction over that text. With **match my voice** on:

- `VOICE_MODE=rerank` (default) - Ollama drafts several replies, the miniGPT scores
  each by token log-probability under your writing, the most you-sounding one wins.
- `VOICE_MODE=rewrite` - Ollama drafts once, the miniGPT continues it in your style.

The miniGPT only knows your corpus; the base model still does the reasoning.

## Accounts

Every request needs a session. Register or log in from the UI, or against the API:

```bash
curl -c jar.txt http://localhost:8080/app/auth/register \
  -d '{"email":"you@example.com","password":"at-least-8-chars"}'
```

The response carries a bearer `token`; the same value is set as an httpOnly
cookie. Sessions, chat history, the message corpus, and the trained voice model
are all scoped per account. Passwords are stored as PBKDF2-SHA256. Users and
tokens live in `backend/data/auth.json`.

## Ollama drop-in

```bash
curl http://localhost:8080/api/chat \
  -H "Authorization: Bearer $TOKEN" -d '{
    "model": "llama3.2",
    "messages": [{"role": "user", "content": "explain goroutines"}],
    "voice": true
  }'
```

## Config

Backend env vars: `LLAMAWEB_ADDR` (`:8080`), `OLLAMA_HOST`
(`http://localhost:11434`), `OLLAMA_MODEL` (`llama3.2`), `ML_HOST`
(`http://localhost:8000`), `VOICE_MODE` (`rerank`), `VOICE_CANDIDATES` (`3`),
`LLAMAWEB_DATA_DIR` (`./data`), `LLAMAWEB_CORPUS_DIR` (`../ml/data`),
`LLAMAWEB_COOKIE_SECURE` (`false`; set `true` when serving over HTTPS).

ml env vars: `MINIGPT_OUT_DIR`, `MINIGPT_DEVICE` (`cpu`/`cuda`),
`MINIGPT_BASE_ENCODING` (`gpt2`).
