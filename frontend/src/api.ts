export type Role = "user" | "assistant" | "system";

export interface Message {
  id: string;
  role: Role;
  content: string;
  created_at: string;
  voiced?: boolean;
  original?: string;
}

export interface SessionSummary {
  id: string;
  title: string;
  created_at: string;
  updated_at: string;
}

export interface Session extends SessionSummary {
  messages: Message[];
}

export interface VoiceStatus {
  trained: boolean;
  vocab_size?: number;
  params?: number;
  steps?: number;
  val_loss?: number | null;
  trained_at?: string;
  corpus_chars?: number;
  corpus_tokens?: number;
  seconds?: number;
  notes?: string[];
}

export interface ModelInfo {
  name: string;
  model: string;
}

async function json<T>(res: Response): Promise<T> {
  if (!res.ok) {
    const body = await res.text();
    throw new Error(body || `${res.status} ${res.statusText}`);
  }
  return res.json() as Promise<T>;
}

export const api = {
  listSessions: () =>
    fetch("/app/sessions").then((r) => json<{ sessions: SessionSummary[] }>(r)),
  getSession: (id: string) => fetch(`/app/sessions/${id}`).then((r) => json<Session>(r)),
  createSession: () =>
    fetch("/app/sessions", { method: "POST" }).then((r) => json<Session>(r)),
  deleteSession: (id: string) => fetch(`/app/sessions/${id}`, { method: "DELETE" }),
  models: () =>
    fetch("/app/models").then((r) => json<{ models: ModelInfo[]; default: string }>(r)),
  voiceStatus: () => fetch("/app/voice/status").then((r) => json<VoiceStatus>(r)),
  trainVoice: (steps?: number) =>
    fetch("/app/voice/train", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ steps: steps ?? 0 }),
    }).then((r) => json<Record<string, unknown>>(r)),
};

export interface ChatEvents {
  onSession?: (sessionId: string) => void;
  onStage?: (stage: string) => void;
  onToken?: (text: string) => void;
  onDone?: (payload: {
    message_id: string;
    text: string;
    original: string;
    voiced: boolean;
  }) => void;
  onError?: (message: string) => void;
}

export async function streamChat(
  body: { session_id?: string; message: string; model?: string; voice: boolean },
  events: ChatEvents,
  signal?: AbortSignal
): Promise<void> {
  const res = await fetch("/app/chat", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
    signal,
  });
  if (!res.ok || !res.body) {
    events.onError?.(await res.text());
    return;
  }
  const reader = res.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";

  const dispatch = (block: string) => {
    let event = "message";
    const dataLines: string[] = [];
    for (const line of block.split("\n")) {
      if (line.startsWith("event:")) event = line.slice(6).trim();
      else if (line.startsWith("data:")) dataLines.push(line.slice(5).trim());
    }
    if (dataLines.length === 0) return;
    const data = JSON.parse(dataLines.join("\n"));
    switch (event) {
      case "session":
        events.onSession?.(data.session_id);
        break;
      case "stage":
        events.onStage?.(data.stage);
        break;
      case "token":
        events.onToken?.(data.text);
        break;
      case "done":
        events.onDone?.(data);
        break;
      case "error":
        events.onError?.(data.message);
        break;
    }
  };

  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true });
    let idx;
    while ((idx = buffer.indexOf("\n\n")) !== -1) {
      const block = buffer.slice(0, idx);
      buffer = buffer.slice(idx + 2);
      if (block.trim()) dispatch(block);
    }
  }
  if (buffer.trim()) dispatch(buffer);
}
