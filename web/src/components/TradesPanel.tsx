import React from "react";
import type { Trade } from "../types";

interface Props {
  trades: Trade[];
}

function shortMint(mint: string): string {
  if (mint.length <= 4) return mint;
  return mint.slice(0, 4);
}

function formatTime(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleTimeString(undefined, {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
}

function formatPct(v: number): string {
  if (!Number.isFinite(v)) return "0.00%";
  const sign = v > 0 ? "+" : "";
  return `${sign}${v.toFixed(2)}%`;
}

function formatBase(amountSmallest: number, decimals = 9): string {
  const factor = 10 ** decimals;
  const v = amountSmallest / factor;
  if (v === 0) return "0.00";
  if (Math.abs(v) < 1e-6) return v.toFixed(6);
  if (Math.abs(v) >= 1e9) return v.toFixed(0);
  return v.toLocaleString("en-US", { minimumFractionDigits: 2, maximumFractionDigits: 6 });
}

export const TradesPanel: React.FC<Props> = ({ trades }) => {
  if (trades.length === 0) {
    return (
      <p className="text-xs text-slate-500 px-2 py-1">
        No trades yet. Once the agent starts trading, recent fills will appear
        here.
      </p>
    );
  }

  const solscan = (sig: string) => `https://solscan.io/tx/${sig}`;

  return (
    <table className="w-full text-left border-collapse text-sm">
      <thead className="sticky top-0 z-10">
        <tr className="text-[10px] uppercase tracking-widest text-slate-500 border-b border-border-dark bg-slate-900/95 backdrop-blur">
          <th className="px-4 sm:px-6 py-3 sm:py-4 font-bold">Time</th>
          <th className="px-4 sm:px-6 py-3 sm:py-4 font-bold">Agent</th>
          <th className="px-4 sm:px-6 py-3 sm:py-4 font-bold">Action</th>
          <th className="px-4 sm:px-6 py-3 sm:py-4 font-bold">Entry (base)</th>
          <th className="px-4 sm:px-6 py-3 sm:py-4 font-bold">Exit (base)</th>
          <th className="px-4 sm:px-6 py-3 sm:py-4 font-bold text-right">
            PnL%
          </th>
          <th className="px-4 sm:px-6 py-3 sm:py-4 font-bold">Tx</th>
          <th className="px-4 sm:px-6 py-3 sm:py-4 font-bold">Fees</th>
          <th className="px-4 sm:px-6 py-3 sm:py-4 font-bold">Reason</th>
        </tr>
      </thead>
      <tbody>
        {trades.map((t, idx) => {
          const pnlClass =
            t.pnlPct > 0
              ? "text-profit"
              : t.pnlPct < 0
              ? "text-loss"
              : "text-slate-300";
          const action =
            t.action === "exit"
              ? "SELL"
              : "BUY";

          return (
            <tr
              key={`${t.time}-${t.agentID}-${idx}`}
              className="border-b border-border-dark hover:bg-slate-900/20 transition-colors"
            >
              <td className="px-4 sm:px-6 py-3 sm:py-4 text-slate-400 font-mono text-xs whitespace-nowrap">
                {formatTime(t.time)}
              </td>
              <td className="px-4 sm:px-6 py-3 sm:py-4 font-medium whitespace-nowrap">
                {t.agentID}
              </td>
              <td className="px-4 sm:px-6 py-3 sm:py-4">
                <span
                  className={`px-2 py-0.5 rounded-full text-[10px] font-bold ${
                    action === "SELL"
                      ? "bg-loss/20 text-loss"
                      : "bg-profit/20 text-profit"
                  }`}
                >
                  {action}
                </span>
              </td>
              <td className="px-4 sm:px-6 py-3 sm:py-4 font-mono text-xs">
                {formatBase(t.entryBaseSmallest)}
              </td>
              <td className="px-4 sm:px-6 py-3 sm:py-4 font-mono text-xs">
                {t.exitBaseSmallest
                  ? formatBase(t.exitBaseSmallest)
                  : "-"}
              </td>
              <td
                className={`px-4 sm:px-6 py-3 sm:py-4 text-right font-bold ${pnlClass}`}
              >
                {formatPct(t.pnlPct)}
              </td>
              <td className="px-4 sm:px-6 py-3 sm:py-4 text-xs whitespace-nowrap">
                {t.action === "exit" && t.exitTxSig ? (
                  <a
                    href={solscan(t.exitTxSig)}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-primary hover:underline font-mono"
                  >
                    tx
                  </a>
                ) : t.entryTxSig ? (
                  <a
                    href={solscan(t.entryTxSig)}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-primary hover:underline font-mono"
                  >
                    tx
                  </a>
                ) : (
                  "-"
                )}
              </td>
              <td className="px-4 sm:px-6 py-3 sm:py-4 text-xs text-slate-400 font-mono whitespace-nowrap">
                {t.feesLamports != null && t.feesLamports > 0
                  ? (t.feesLamports / 1e9).toFixed(6)
                  : "-"}
              </td>
              <td className="px-4 sm:px-6 py-3 sm:py-4 text-xs text-slate-400 whitespace-nowrap">
                {t.reason || `${shortMint(t.baseMint)}→${shortMint(
                  t.targetMint
                )}`}
              </td>
            </tr>
          );
        })}
      </tbody>
    </table>
  );
};


