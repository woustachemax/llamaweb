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

export interface User {
  id: string;
  email: string;
}

export class AuthError extends Error {}

async function req<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, { credentials: "include", ...init });
  if (res.status === 401) {
    throw new AuthError("not signed in");
  }
  if (!res.ok) {
    let message = `${res.status} ${res.statusText}`;
    try {
      const body = await res.json();
      if (body && typeof body.error === "string") message = body.error;
    } catch {
      /* keep default */
    }
    throw new Error(message);
  }
  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}

function jsonBody(body: unknown): RequestInit {
  return {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  };
}

export const api = {
  me: () => req<{ user: User }>("/app/auth/me").then((r) => r.user),
  register: (email: string, password: string) =>
    req<{ user: User }>("/app/auth/register", jsonBody({ email, password })).then((r) => r.user),
  login: (email: string, password: string) =>
    req<{ user: User }>("/app/auth/login", jsonBody({ email, password })).then((r) => r.user),
  logout: () => req<void>("/app/auth/logout", { method: "POST" }),

  listSessions: () => req<{ sessions: SessionSummary[] }>("/app/sessions"),
  getSession: (id: string) => req<Session>(`/app/sessions/${id}`),
  createSession: () => req<Session>("/app/sessions", { method: "POST" }),
  deleteSession: (id: string) => req<void>(`/app/sessions/${id}`, { method: "DELETE" }),
  models: () => req<{ models: ModelInfo[]; default: string }>("/app/models"),
  voiceStatus: () => req<VoiceStatus>("/app/voice/status"),
  trainVoice: (steps?: number) =>
    req<Record<string, unknown>>("/app/voice/train", jsonBody({ steps: steps ?? 0 })),
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
  onUnauthorized?: () => void;
}

export async function streamChat(
  body: { session_id?: string; message: string; model?: string; voice: boolean },
  events: ChatEvents,
  signal?: AbortSignal
): Promise<void> {
  const res = await fetch("/app/chat", {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
    signal,
  });
  if (res.status === 401) {
    events.onUnauthorized?.();
    return;
  }
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
