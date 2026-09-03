import { useState } from "react";
import { api, VoiceStatus } from "../api";

interface Props {
  status: VoiceStatus | null;
  onRefresh: () => void;
}

export default function VoicePanel({ status, onRefresh }: Props) {
  const [training, setTraining] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [last, setLast] = useState<string | null>(null);

  const train = async () => {
    setTraining(true);
    setError(null);
    try {
      const res = await api.trainVoice();
      setLast(
        `trained ${res.steps ?? "?"} steps · val loss ${res.val_loss ?? "?"} · ${
          res.seconds ?? "?"
        }s`
      );
      onRefresh();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setTraining(false);
    }
  };

  return (
    <div className="voice-panel">
      <div className="voice-head">
        <strong>Voice model</strong>
        <span className={"dot " + (status?.trained ? "ok" : "off")} />
      </div>
      {status?.trained ? (
        <ul className="voice-stats">
          <li>vocab {status.vocab_size}</li>
          <li>params {status.params?.toLocaleString()}</li>
          <li>steps {status.steps}</li>
          <li>val loss {status.val_loss ?? "n/a"}</li>
          <li>corpus {status.corpus_tokens ?? status.corpus_chars ?? 0} tok</li>
        </ul>
      ) : (
        <p className="voice-hint">
          Not trained yet. Chat a bit so it can learn your phrasing, then train.
        </p>
      )}
      <button className="train-btn" disabled={training} onClick={train}>
        {training ? "training…" : status?.trained ? "retrain on my messages" : "train on my messages"}
      </button>
      {last && <p className="voice-ok">{last}</p>}
      {error && <p className="voice-err">{error}</p>}
    </div>
  );
}
