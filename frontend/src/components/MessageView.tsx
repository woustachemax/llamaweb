import { useState } from "react";
import { Message } from "../api";

export default function MessageView({ message }: { message: Message }) {
  const [showOriginal, setShowOriginal] = useState(false);
  const isUser = message.role === "user";
  const body =
    showOriginal && message.original ? message.original : message.content;

  return (
    <div className={"msg " + (isUser ? "msg-user" : "msg-assistant")}>
      <div className="msg-role">{isUser ? "you" : "assistant"}</div>
      <div className="msg-body">{body || "…"}</div>
      {!isUser && message.voiced && message.original && (
        <button className="voice-toggle" onClick={() => setShowOriginal((v) => !v)}>
          {showOriginal ? "show voiced reply" : "show base draft"}
        </button>
      )}
      {!isUser && message.voiced && <span className="voiced-badge">voice-matched</span>}
    </div>
  );
}
