import { useCallback, useEffect, useRef, useState } from "react";
import {
  api,
  AuthError,
  Message,
  ModelInfo,
  SessionSummary,
  streamChat,
  User,
  VoiceStatus,
} from "./api";
import Auth from "./components/Auth";
import Sidebar from "./components/Sidebar";
import MessageView from "./components/MessageView";
import VoicePanel from "./components/VoicePanel";

export default function App() {
  const [user, setUser] = useState<User | null>(null);
  const [authChecked, setAuthChecked] = useState(false);
  const [sessions, setSessions] = useState<SessionSummary[]>([]);
  const [activeId, setActiveId] = useState<string | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState("");
  const [busy, setBusy] = useState(false);
  const [stage, setStage] = useState<string | null>(null);
  const [voice, setVoice] = useState(true);
  const [models, setModels] = useState<ModelInfo[]>([]);
  const [model, setModel] = useState("");
  const [voiceStatus, setVoiceStatus] = useState<VoiceStatus | null>(null);
  const [error, setError] = useState<string | null>(null);
  const scrollRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    api
      .me()
      .then(setUser)
      .catch(() => setUser(null))
      .finally(() => setAuthChecked(true));
  }, []);

  const signOut = useCallback(() => {
    setUser(null);
    setSessions([]);
    setActiveId(null);
    setMessages([]);
    setVoiceStatus(null);
  }, []);

  const guard = useCallback(
    (e: unknown) => {
      if (e instanceof AuthError) {
        signOut();
        return;
      }
      setError(e instanceof Error ? e.message : String(e));
    },
    [signOut]
  );

  const refreshSessions = useCallback(async () => {
    try {
      const { sessions } = await api.listSessions();
      setSessions(sessions);
    } catch (e) {
      guard(e);
    }
  }, [guard]);

  const refreshVoice = useCallback(async () => {
    try {
      setVoiceStatus(await api.voiceStatus());
    } catch (e) {
      if (e instanceof AuthError) signOut();
      else setVoiceStatus(null);
    }
  }, [signOut]);

  useEffect(() => {
    if (!user) return;
    refreshSessions();
    refreshVoice();
    api
      .models()
      .then((m) => {
        setModels(m.models);
        setModel(m.default);
      })
      .catch((e) => {
        if (e instanceof AuthError) signOut();
      });
  }, [user, refreshSessions, refreshVoice, signOut]);

  useEffect(() => {
    if (!activeId) {
      setMessages([]);
      return;
    }
    api
      .getSession(activeId)
      .then((s) => setMessages(s.messages))
      .catch(guard);
  }, [activeId, guard]);

  useEffect(() => {
    scrollRef.current?.scrollTo({ top: scrollRef.current.scrollHeight });
  }, [messages, stage]);

  const logout = async () => {
    try {
      await api.logout();
    } catch {
      /* clearing local state regardless */
    }
    signOut();
  };

  const newChat = async () => {
    try {
      const s = await api.createSession();
      await refreshSessions();
      setActiveId(s.id);
      setMessages([]);
    } catch (e) {
      guard(e);
    }
  };

  const deleteChat = async (id: string) => {
    try {
      await api.deleteSession(id);
    } catch (e) {
      guard(e);
      return;
    }
    if (id === activeId) {
      setActiveId(null);
      setMessages([]);
    }
    refreshSessions();
  };

  const send = async () => {
    const text = input.trim();
    if (!text || busy) return;
    setInput("");
    setBusy(true);
    setError(null);
    setStage(null);

    const userMsg: Message = {
      id: "local-" + Date.now(),
      role: "user",
      content: text,
      created_at: new Date().toISOString(),
    };
    const assistantMsg: Message = {
      id: "local-assistant-" + Date.now(),
      role: "assistant",
      content: "",
      created_at: new Date().toISOString(),
    };
    setMessages((m) => [...m, userMsg, assistantMsg]);

    let streamed = "";
    await streamChat(
      {
        session_id: activeId ?? undefined,
        message: text,
        model: model || undefined,
        voice,
      },
      {
        onUnauthorized: signOut,
        onSession: (id) => {
          if (!activeId) {
            setActiveId(id);
            refreshSessions();
          }
        },
        onStage: (s) => setStage(s),
        onToken: (t) => {
          streamed += t;
          setMessages((m) => {
            const copy = [...m];
            copy[copy.length - 1] = { ...copy[copy.length - 1], content: streamed };
            return copy;
          });
        },
        onDone: (payload) => {
          setMessages((m) => {
            const copy = [...m];
            copy[copy.length - 1] = {
              ...copy[copy.length - 1],
              id: payload.message_id,
              content: payload.text,
              original: payload.original,
              voiced: payload.voiced,
            };
            return copy;
          });
          setStage(null);
          refreshSessions();
        },
        onError: (msg) => setError(msg),
      }
    ).catch((e) => setError(e instanceof Error ? e.message : String(e)));

    setBusy(false);
    setStage(null);
  };

  if (!authChecked) {
    return <div className="boot">loading…</div>;
  }

  if (!user) {
    return <Auth onAuthed={setUser} />;
  }

  return (
    <div className="layout">
      <Sidebar
        sessions={sessions}
        activeId={activeId}
        onSelect={setActiveId}
        onNew={newChat}
        onDelete={deleteChat}
      />
      <main className="main">
        <header className="topbar">
          <div className="brand">llamaweb</div>
          <div className="controls">
            <label className="voice-switch">
              <input
                type="checkbox"
                checked={voice}
                onChange={(e) => setVoice(e.target.checked)}
              />
              match my voice
            </label>
            <select value={model} onChange={(e) => setModel(e.target.value)}>
              {models.length === 0 && <option value="">default</option>}
              {models.map((m) => (
                <option key={m.name} value={m.model || m.name}>
                  {m.name}
                </option>
              ))}
            </select>
            <span className="who">{user.email}</span>
            <button className="logout" onClick={logout}>
              sign out
            </button>
          </div>
        </header>

        <div className="chat-scroll" ref={scrollRef}>
          {messages.length === 0 && (
            <div className="welcome">
              <h1>Your self-hosted LLM, in the browser</h1>
              <p>
                Runs an Ollama model through a Go agent, then a PyTorch miniGPT
                re-ranks the reply so it sounds like you.
              </p>
            </div>
          )}
          {messages.map((m) => (
            <MessageView key={m.id} message={m} />
          ))}
          {stage && <div className="stage">{stage}…</div>}
        </div>

        {error && <div className="error-bar">{error}</div>}

        <div className="composer">
          <textarea
            value={input}
            placeholder="Send a message"
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter" && !e.shiftKey) {
                e.preventDefault();
                send();
              }
            }}
            rows={1}
          />
          <button disabled={busy || !input.trim()} onClick={send}>
            {busy ? "…" : "send"}
          </button>
        </div>
      </main>

      <div className="rail">
        <VoicePanel status={voiceStatus} onRefresh={refreshVoice} />
      </div>
    </div>
  );
}
