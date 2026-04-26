import React, { useState } from "react";
import { createAgent, type CreateAgentPayload } from "../api/client";

interface Props {
  onCreated: () => void;
}

const TP_SL_MIN = 0;
const TP_SL_MAX = 100;

export const CreateAgentForm: React.FC<Props> = ({ onCreated }) => {
  const [form, setForm] = useState({
    id: "",
    baseMint: "So11111111111111111111111111111111111111112",
    targetMint: "",
    mode: "paper" as "paper" | "live",
    tradeSizeLamports: 100_000_000,
    pollSeconds: 5,
    cooldownSeconds: 60,
    maxSlippageBps: 100,
    takeProfitPct: 5,
    stopLossPct: 2,
    maxConsecutiveLosses: 3,
    maxTotalLossSOL: 0.03,
    maxPriceImpactPct: 1,
    minOutTargetSmallest: 0,
    quoteStabilityWindow: 0,
    maxJitterPct: 0.5
  });
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [advancedOpen, setAdvancedOpen] = useState(true);

  function update<K extends keyof typeof form>(key: K, value: typeof form[K]) {
    setForm((f) => ({ ...f, [key]: value }));
  }

  const takeProfitValid = form.takeProfitPct >= TP_SL_MIN && form.takeProfitPct <= TP_SL_MAX;
  const stopLossValid = form.stopLossPct >= TP_SL_MIN && form.stopLossPct <= TP_SL_MAX;
  const canSubmit = takeProfitValid && stopLossValid;

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!canSubmit) return;
    setSubmitting(true);
    setError(null);
    const payload: CreateAgentPayload = {
      ...form,
      id: form.id || undefined,
      mode: form.mode,
      minOutTargetSmallest: form.minOutTargetSmallest || undefined,
      quoteStabilityWindow: form.quoteStabilityWindow || undefined,
      maxJitterPct: form.maxJitterPct || undefined
    };
    try {
      await createAgent(payload);
      onCreated();
    } catch (err: any) {
      setError(err?.message ?? String(err));
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-4 text-sm">
      {/* Token settings (ID / mints) */}
      <div>
        <h3 className="text-xs font-bold uppercase tracking-widest text-slate-500 mb-2">
          Token & Agent Settings
        </h3>
        <div className="grid grid-cols-1 gap-3">
          <div className="space-y-2">
            <label className="text-[11px] font-medium text-slate-300">
              Agent ID (optional)
            </label>
            <input
              className="w-full bg-slate-900 border border-border-dark rounded-lg px-3 py-2 text-sm focus:ring-1 focus:ring-primary focus:border-primary outline-none"
              placeholder="auto-generated if empty"
              value={form.id}
              onChange={(e) => update("id", e.target.value)}
            />
          </div>
          <div className="space-y-2">
            <label className="text-[11px] font-medium text-slate-300">
              Base Mint (SOL)
            </label>
            <input
              className="w-full bg-slate-900 border border-border-dark rounded-lg px-3 py-2 text-sm focus:ring-1 focus:ring-primary focus:border-primary outline-none font-mono"
              value={form.baseMint}
              onChange={(e) => update("baseMint", e.target.value)}
            />
          </div>
          <div className="space-y-2">
            <label className="text-[11px] font-medium text-slate-300">
              Target Mint
            </label>
            <input
              className="w-full bg-slate-900 border border-border-dark rounded-lg px-3 py-2 text-sm focus:ring-1 focus:ring-primary focus:border-primary outline-none font-mono"
              placeholder="Target SPL mint"
              value={form.targetMint}
              onChange={(e) => update("targetMint", e.target.value)}
            />
          </div>
          <div className="space-y-2">
            <label className="text-[11px] font-medium text-slate-300">
              Mode
            </label>
            <select
              className="w-full bg-slate-900 border border-border-dark rounded-lg px-3 py-2 text-sm focus:ring-1 focus:ring-primary focus:border-primary outline-none"
              value={form.mode}
              onChange={(e) => update("mode", e.target.value as "paper" | "live")}
            >
              <option value="paper">Paper</option>
              <option value="live">Live</option>
            </select>
            <p className="text-[10px] text-slate-500">
              Paper: simulated; Live: real swaps (requires wallet).
            </p>
          </div>
        </div>
      </div>

      {/* Trading settings */}
      <div>
        <h3 className="text-xs font-bold uppercase tracking-widest text-slate-500 mb-2">
          Trading Settings
        </h3>
        <div className="grid grid-cols-2 gap-3">
          <div className="space-y-2">
            <label className="text-[11px] font-medium text-slate-300">
              Trade Size (SOL)
            </label>
            <input
              className="w-full bg-slate-900 border border-border-dark rounded-lg px-3 py-2 text-sm focus:ring-1 focus:ring-primary focus:border-primary outline-none"
              placeholder="0.1"
              type="number"
              step="0.000000001"
              value={form.tradeSizeLamports / 1_000_000_000}
              onChange={(e) =>
                update(
                  "tradeSizeLamports",
                  Math.round(Number(e.target.value || 0) * 1_000_000_000)
                )
              }
            />
          </div>
          <div className="space-y-2">
            <label className="text-[11px] font-medium text-slate-300">
              Slippage (bps)
            </label>
            <input
              className="w-full bg-slate-900 border border-border-dark rounded-lg px-3 py-2 text-sm focus:ring-1 focus:ring-primary focus:border-primary outline-none"
              placeholder="100"
              type="number"
              value={form.maxSlippageBps}
              onChange={(e) =>
                update("maxSlippageBps", Number(e.target.value || 0))
              }
            />
          </div>
        </div>
      </div>

      {/* Timing settings */}
      <div>
        <h3 className="text-xs font-bold uppercase tracking-widest text-slate-500 mb-2">
          Timing Settings
        </h3>
        <div className="grid grid-cols-2 gap-3">
          <div className="space-y-2">
            <label className="text-[11px] font-medium text-slate-300">
              Poll Interval (sec)
            </label>
            <input
              className="w-full bg-slate-900 border border-border-dark rounded-lg px-3 py-2 text-sm focus:ring-1 focus:ring-primary focus:border-primary outline-none"
              placeholder="5"
              type="number"
              value={form.pollSeconds}
              onChange={(e) =>
                update("pollSeconds", Number(e.target.value || 0))
              }
            />
          </div>
          <div className="space-y-2">
            <label className="text-[11px] font-medium text-slate-300">
              Cooldown (sec)
            </label>
            <input
              className="w-full bg-slate-900 border border-border-dark rounded-lg px-3 py-2 text-sm focus:ring-1 focus:ring-primary focus:border-primary outline-none"
              placeholder="60"
              type="number"
              value={form.cooldownSeconds}
              onChange={(e) =>
                update("cooldownSeconds", Number(e.target.value || 0))
              }
            />
          </div>
        </div>
      </div>

      {/* Risk settings */}
      <div>
        <h3 className="text-xs font-bold uppercase tracking-widest text-slate-500 mb-2">
          Risk Settings
        </h3>
        <div className="grid grid-cols-2 gap-3">
          <div className="space-y-2">
            <label className="text-[11px] font-medium text-slate-300">
              Take Profit (%)
            </label>
            <input
              className={`w-full bg-slate-900 border rounded-lg px-3 py-2 text-sm focus:ring-1 focus:ring-primary outline-none text-profit ${
                !takeProfitValid ? "border-loss" : "border-border-dark focus:border-primary"
              }`}
              placeholder="5"
              type="number"
              min={TP_SL_MIN}
              max={TP_SL_MAX}
              step="0.1"
              value={form.takeProfitPct}
              onChange={(e) =>
                update("takeProfitPct", Number(e.target.value ?? 0))
              }
            />
            <p className="text-[10px] text-slate-500">Percent, e.g. 5 = 5%</p>
          </div>
          <div className="space-y-2">
            <label className="text-[11px] font-medium text-slate-300">
              Stop Loss (%)
            </label>
            <input
              className={`w-full bg-slate-900 border rounded-lg px-3 py-2 text-sm focus:ring-1 focus:ring-primary outline-none text-loss ${
                !stopLossValid ? "border-loss" : "border-border-dark focus:border-primary"
              }`}
              placeholder="2"
              type="number"
              min={TP_SL_MIN}
              max={TP_SL_MAX}
              step="0.1"
              value={form.stopLossPct}
              onChange={(e) =>
                update("stopLossPct", Number(e.target.value ?? 0))
              }
            />
            <p className="text-[10px] text-slate-500">Percent, e.g. 2 = 2%</p>
          </div>
        </div>
      </div>

      {/* Advanced (collapsible, open by default) */}
      <div className="border border-border-dark rounded-xl overflow-hidden bg-slate-900/30">
        <button
          type="button"
          onClick={() => setAdvancedOpen((o) => !o)}
          className="w-full px-4 py-3 flex items-center justify-between text-left text-xs font-bold uppercase tracking-widest text-slate-500 hover:bg-slate-800/50"
        >
          Advanced
          <span className="material-symbols-outlined text-base">
            {advancedOpen ? "expand_less" : "expand_more"}
          </span>
        </button>
        {advancedOpen && (
          <div className="p-4 pt-0 space-y-4">
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-2">
                <label className="text-[11px] font-medium text-slate-300">
                  Max Price Impact (%)
                </label>
                <input
                  className="w-full bg-slate-900 border border-border-dark rounded-lg px-3 py-2 text-sm focus:ring-1 focus:ring-primary outline-none"
                  type="number"
                  value={form.maxPriceImpactPct}
                  onChange={(e) =>
                    update("maxPriceImpactPct", Number(e.target.value || 0))
                  }
                />
              </div>
              <div className="space-y-2">
                <label className="text-[11px] font-medium text-slate-300">
                  Max Consecutive Losses
                </label>
                <input
                  className="w-full bg-slate-900 border border-border-dark rounded-lg px-3 py-2 text-sm focus:ring-1 focus:ring-primary outline-none"
                  type="number"
                  value={form.maxConsecutiveLosses}
                  onChange={(e) =>
                    update("maxConsecutiveLosses", Number(e.target.value || 0))
                  }
                />
              </div>
              <div className="space-y-2">
                <label className="text-[11px] font-medium text-slate-300">
                  Max Total Loss (SOL)
                </label>
                <input
                  className="w-full bg-slate-900 border border-border-dark rounded-lg px-3 py-2 text-sm focus:ring-1 focus:ring-primary outline-none"
                  type="number"
                  step="0.001"
                  value={form.maxTotalLossSOL}
                  onChange={(e) =>
                    update("maxTotalLossSOL", Number(e.target.value || 0))
                  }
                />
              </div>
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-2">
                <label className="text-[11px] font-medium text-slate-300">
                  Min out (target smallest)
                </label>
                <input
                  className="w-full bg-slate-900 border border-border-dark rounded-lg px-3 py-2 text-sm focus:ring-1 focus:ring-primary outline-none font-mono"
                  type="number"
                  placeholder="0 = off"
                  value={form.minOutTargetSmallest === 0 ? "" : form.minOutTargetSmallest}
                  onChange={(e) =>
                    update("minOutTargetSmallest", Number(e.target.value || 0))
                  }
                />
              </div>
              <div className="space-y-2">
                <label className="text-[11px] font-medium text-slate-300">
                  Quote stability window
                </label>
                <input
                  className="w-full bg-slate-900 border border-border-dark rounded-lg px-3 py-2 text-sm focus:ring-1 focus:ring-primary outline-none"
                  type="number"
                  min={0}
                  placeholder="0 = off"
                  value={form.quoteStabilityWindow === 0 ? "" : form.quoteStabilityWindow}
                  onChange={(e) =>
                    update("quoteStabilityWindow", Number(e.target.value || 0))
                  }
                />
              </div>
            </div>
            <div className="space-y-2">
              <label className="text-[11px] font-medium text-slate-300">
                Max jitter (%)
              </label>
              <input
                className="w-full bg-slate-900 border border-border-dark rounded-lg px-3 py-2 text-sm focus:ring-1 focus:ring-primary outline-none"
                type="number"
                step="0.1"
                placeholder="0.5"
                value={form.maxJitterPct === 0 ? "" : form.maxJitterPct}
                onChange={(e) =>
                  update("maxJitterPct", Number(e.target.value || 0))
                }
              />
            </div>
          </div>
        )}
      </div>

      {/* Preview: values sent to backend */}
      <div className="bg-slate-900/50 border border-border-dark rounded-lg p-3 font-mono text-[11px] text-slate-400 space-y-1">
        <p className="text-slate-500 font-bold uppercase tracking-wider mb-2">
          Computed values sent to backend
        </p>
        <p>tradeSizeLamports: {form.tradeSizeLamports}</p>
        <p>maxSlippageBps: {form.maxSlippageBps}</p>
        <p>pollSeconds: {form.pollSeconds}</p>
        <p>cooldownSeconds: {form.cooldownSeconds}</p>
        <p>takeProfitPct: {form.takeProfitPct}%</p>
        <p>stopLossPct: {form.stopLossPct}%</p>
      </div>

      {error && (
        <div className="text-xs text-red-400 bg-red-900/40 border border-red-500/40 rounded-lg px-3 py-2">
          Error: {error}
        </div>
      )}

      <button
        type="submit"
        disabled={submitting || !canSubmit}
        className="w-full bg-primary hover:bg-primary/90 text-white font-bold py-3 rounded-lg transition-colors flex items-center justify-center gap-2 disabled:opacity-60 disabled:cursor-not-allowed"
      >
        <span className="material-symbols-outlined">rocket_launch</span>
        {submitting ? "Creating..." : "Create Agent"}
      </button>
    </form>
  );
};

