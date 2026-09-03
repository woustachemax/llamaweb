import { SessionSummary } from "../api";

interface Props {
  sessions: SessionSummary[];
  activeId: string | null;
  onSelect: (id: string) => void;
  onNew: () => void;
  onDelete: (id: string) => void;
}

export default function Sidebar({ sessions, activeId, onSelect, onNew, onDelete }: Props) {
  return (
    <aside className="sidebar">
      <button className="new-chat" onClick={onNew}>
        + New chat
      </button>
      <div className="session-list">
        {sessions.map((s) => (
          <div
            key={s.id}
            className={"session-item" + (s.id === activeId ? " active" : "")}
            onClick={() => onSelect(s.id)}
          >
            <span className="session-title">{s.title || "New chat"}</span>
            <button
              className="session-delete"
              onClick={(e) => {
                e.stopPropagation();
                onDelete(s.id);
              }}
              aria-label="delete"
            >
              ×
            </button>
          </div>
        ))}
        {sessions.length === 0 && <p className="empty">No chats yet</p>}
      </div>
    </aside>
  );
}
