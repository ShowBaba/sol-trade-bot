import React, { useMemo } from "react";
import type { AgentSnapshot, Trade } from "../types";
import { startAgent, stopAgent, deleteAgent, resetAgent } from "../api/client";

interface Props {
  agents: AgentSnapshot[];
  trades?: Trade[];
  onChanged: () => void;
}

function agentStats(trades: Trade[], agentID: string) {
  const exits = trades.filter(
    (t) => t.agentID === agentID && t.action === "exit"
  );
  const wins = exits.filter((t) => t.pnlPct > 0).length;
  const losses = exits.filter((t) => t.pnlPct < 0).length;
  const last = exits.length
    ? new Date(
        Math.max(...exits.map((t) => new Date(t.time).getTime()))
      ).toLocaleString(undefined, {
        month: "short",
        day: "numeric",
        hour: "2-digit",
        minute: "2-digit"
      })
    : null;
  const total = wins + losses;
  const winRate = total > 0 ? ((wins / total) * 100).toFixed(0) : null;
  return { wins, losses, lastTrade: last, winRate };
}

function shortMint(mint: string): string {
  if (mint.length <= 8) return mint;
  return `${mint.slice(0, 4)}…${mint.slice(-3)}`;
}

export const AgentsPanel: React.FC<Props> = ({
  agents,
  trades = [],
  onChanged
}) => {
  async function wrap(action: () => Promise<void>) {
    try {
      await action();
      onChanged();
    } catch (err: any) {
      alert(err?.message ?? String(err));
    }
  }

  const btnBase =
    "inline-flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg text-xs font-medium transition-colors focus:outline-none focus:ring-2 focus:ring-offset-0 focus:ring-offset-slate-900 disabled:opacity-50 disabled:cursor-not-allowed disabled:pointer-events-none";

  const statsByAgent = useMemo(() => {
    const m = new Map<string, ReturnType<typeof agentStats>>();
    agents.forEach((a) => m.set(a.id, agentStats(trades, a.id)));
    return m;
  }, [agents, trades]);

  return (
    <div className="space-y-4">
      {agents.map((a) => {
        const stats = statsByAgent.get(a.id)!;
        const tradeSize =
          a.tradeSizeBase ??
          a.tradeSizeBaseSmallestDisplay ??
          undefined;
        const pnl = a.totalPnLBase ?? 0;
        const pnlClass =
          pnl > 0 ? "text-profit" : pnl < 0 ? "text-loss" : "text-slate-400";
        const statusClass =
          a.status === "stopped"
            ? "bg-slate-600/50 text-slate-400"
            : a.status === "needs_manual_attention"
              ? "bg-red-500/20 text-red-400"
              : a.status === "in_position"
                ? "bg-amber-500/20 text-amber-400"
                : "bg-profit/20 text-profit";
        const lastExitTrade = trades
          .filter((t) => t.agentID === a.id && t.action === "exit")
          .sort(
            (x, y) =>
              new Date(y.time).getTime() - new Date(x.time).getTime()
          )[0];
        const solscan = (sig: string) =>
          `https://solscan.io/tx/${sig}`;
        return (
          <div
            key={a.id}
            className="bg-slate-800/60 border border-border-dark rounded-xl p-4 space-y-3"
          >
            <div className="flex flex-wrap items-center justify-between gap-2">
              <div className="flex items-center gap-2">
                <span className="font-mono font-semibold text-slate-200">
                  {a.id}
                </span>
                <span
                  className={
                    "inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium " +
                    statusClass
                  }
                >
                  {a.status.replace(/_/g, " ")}
                </span>
                {(a.mode || "").toLowerCase() === "live" && (
                  <span className="text-[10px] text-amber-400 font-semibold uppercase tracking-wider">
                    Live
                  </span>
                )}
              </div>
              <div className="flex flex-wrap items-center gap-1.5">
                <button
                  onClick={() => wrap(() => startAgent(a.id))}
                  className={`${btnBase} bg-profit/20 text-profit hover:bg-profit/30 focus:ring-profit/50`}
                  title="Start agent"
                >
                  <span className="material-symbols-outlined text-sm">
                    play_arrow
                  </span>
                  Start
                </button>
                <button
                  onClick={() => wrap(() => stopAgent(a.id))}
                  className={`${btnBase} bg-amber-500/20 text-amber-400 hover:bg-amber-500/30 focus:ring-amber-500/50`}
                  title="Stop agent"
                >
                  <span className="material-symbols-outlined text-sm">
                    stop
                  </span>
                  Stop
                </button>
                <button
                  onClick={() => wrap(() => resetAgent(a.id))}
                  className={`${btnBase} bg-slate-600/50 text-slate-300 hover:bg-slate-600 focus:ring-slate-500`}
                  title="Reset stats"
                >
                  <span className="material-symbols-outlined text-sm">
                    restart_alt
                  </span>
                  Reset stats
                </button>
                <button
                  onClick={() => wrap(() => deleteAgent(a.id))}
                  disabled={a.status !== "stopped"}
                  className={`${btnBase} bg-loss/20 text-loss hover:bg-loss/30 focus:ring-loss/50`}
                  title={
                    a.status !== "stopped"
                      ? "Stop the agent first"
                      : "Delete agent"
                  }
                >
                  <span className="material-symbols-outlined text-sm">
                    delete
                  </span>
                  Delete
                </button>
              </div>
            </div>
            <div className="grid grid-cols-2 sm:grid-cols-3 gap-x-4 gap-y-1 text-xs text-slate-400 font-mono">
              <span>
                {shortMint(a.baseMint)} → {shortMint(a.targetMint)}
              </span>
              <span>Size: {tradeSize ?? "?"}</span>
              <span>TP/SL: {a.takeProfitPct ?? "?"}% / {a.stopLossPct ?? "?"}%</span>
              <span>Losses: {a.consecutiveLosses}</span>
              <span className={pnlClass}>PnL: {pnl.toFixed(6)}</span>
              {(stats.wins > 0 || stats.losses > 0) && (
                <>
                  <span>W/L: {stats.wins}/{stats.losses}</span>
                  {stats.winRate != null && (
                    <span>Win rate: {stats.winRate}%</span>
                  )}
                  {stats.lastTrade != null && (
                    <span>Last: {stats.lastTrade}</span>
                  )}
                </>
              )}
            </div>
            {(a.position?.entryTxSig || lastExitTrade?.exitTxSig || lastExitTrade?.entryTxSig) && (
              <div className="flex flex-wrap items-center gap-3 text-xs">
                {a.position?.entryTxSig && (
                  <a
                    href={solscan(a.position.entryTxSig)}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-primary hover:underline font-mono"
                  >
                    Entry tx
                  </a>
                )}
                {lastExitTrade?.exitTxSig && (
                  <a
                    href={solscan(lastExitTrade.exitTxSig)}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-primary hover:underline font-mono"
                  >
                    Exit tx
                  </a>
                )}
              </div>
            )}
            {a.lastError && (
              <p className="text-xs text-red-400 bg-red-900/30 rounded px-2 py-1">
                {a.lastError}
              </p>
            )}
          </div>
        );
      })}
      {agents.length === 0 && (
        <p className="text-slate-500 text-sm text-center py-6">No agents yet.</p>
      )}
    </div>
  );
};

