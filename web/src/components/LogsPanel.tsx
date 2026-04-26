import React, { useEffect, useRef, useState } from "react";
import type { LogEvent } from "../types";
import {
  fetchLogHistory,
  createLogsEventSource,
  clearLogs
} from "../api/client";

export const LogsPanel: React.FC = () => {
  const [logs, setLogs] = useState<LogEvent[]>([]);
  const [autoScroll, setAutoScroll] = useState(true);
  const scrollRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    let es: EventSource | null = null;
    (async () => {
      try {
        const hist = await fetchLogHistory();
        setLogs(hist);
      } catch (err) {
        console.error("failed to load log history", err);
      }
      es = createLogsEventSource((e) => {
        setLogs((prev) => [...prev, e]);
      });
    })();
    return () => {
      if (es) es.close();
    };
  }, []);

  useEffect(() => {
    if (autoScroll && scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [logs, autoScroll]);

  async function handleClear() {
    try {
      await clearLogs();
      setLogs([]);
    } catch (err) {
      console.error("failed to clear logs", err);
    }
  }

  return (
    <>
      <div className="p-4 border-b border-border-dark flex items-center justify-between">
        <h2 className="font-bold flex items-center gap-2 text-sm">
          <span className="material-symbols-outlined text-slate-400 text-base">
            terminal
          </span>
          Live Logs
        </h2>

        <div className="flex items-center gap-4">
          <label className="flex items-center gap-2 cursor-pointer select-none">
            <input
              type="checkbox"
              checked={autoScroll}
              onChange={(e) => setAutoScroll(e.target.checked)}
              className="rounded border-slate-700 bg-slate-800 text-primary focus:ring-0 focus:ring-offset-0"
              aria-label="Auto-scroll"
            />
            <span className="text-[10px] font-bold uppercase tracking-wider text-slate-400">
              Auto-scroll
            </span>
          </label>
          <button
            type="button"
            className="text-slate-500 hover:text-slate-300"
            aria-label="Clear logs"
            onClick={handleClear}
          >
            <span className="material-symbols-outlined text-base">
              delete_sweep
            </span>
          </button>
        </div>
      </div>

      <div
        ref={scrollRef}
        className="p-4 font-mono text-[11px] leading-relaxed custom-scrollbar overflow-y-auto flex-grow h-64"
      >
        {logs.map((l, idx) => (
          <div key={idx} className="text-slate-300">
            {typeof l.time === "string" ? l.time : ""} [{l.level}] ({l.agent}){" "}
            {l.msg} {l.fields ? JSON.stringify(l.fields) : ""}
          </div>
        ))}
      </div>
    </>
  );
};
