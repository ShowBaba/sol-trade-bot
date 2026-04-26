// App.tsx
import React, { useEffect, useMemo, useRef, useState } from "react";
import { CreateAgentForm } from "./components/CreateAgentForm";
import { AgentsPanel } from "./components/AgentsPanel";
import { TradesPanel } from "./components/TradesPanel";
import { LogsPanel } from "./components/LogsPanel";
import { fetchAgents, fetchTrades, fetchWallet, type WalletInfo } from "./api/client";
import type { AgentSnapshot, Trade } from "./types";

function cx(...parts: Array<string | false | null | undefined>) {
  return parts.filter(Boolean).join(" ");
}

function shortMint(mint?: string, left = 4, right = 4) {
  if (!mint) return "";
  if (mint.length <= left + right + 3) return mint;
  return `${mint.slice(0, left)}…${mint.slice(-right)}`;
}

function formatSigned(n: number, decimals = 6) {
  const sign = n > 0 ? "+" : n < 0 ? "-" : "";
  return `${sign}${Math.abs(n).toFixed(decimals)}`;
}

export const App: React.FC = () => {
  const [agents, setAgents] = useState<AgentSnapshot[]>([]);
  const [trades, setTrades] = useState<Trade[]>([]);
  const [wallet, setWallet] = useState<WalletInfo | null>(null);
  const [tradesAutoScroll, setTradesAutoScroll] = useState(true);
  const tradesScrollRef = useRef<HTMLDivElement>(null);

  async function refreshAgents() {
    try {
      const list = await fetchAgents();
      setAgents(list);
    } catch (err: any) {
      console.error("failed to fetch agents", err);
      alert(err?.message ?? String(err));
    }
  }

  async function refreshTrades() {
    try {
      const list = await fetchTrades();
      setTrades(list);
    } catch (err) {
      console.error("failed to fetch trades", err);
    }
  }

  async function refreshWallet() {
    try {
      const w = await fetchWallet();
      setWallet(w.pubkey ? w : null);
    } catch (err) {
      console.warn("Wallet fetch failed:", err);
      setWallet(null);
    }
  }

  useEffect(() => {
    void refreshAgents();
    void refreshTrades();
    void refreshWallet();
    const id = window.setInterval(() => {
      void refreshAgents();
      void refreshTrades();
      void refreshWallet();
    }, 10_000);
    return () => window.clearInterval(id);
  }, []);

  const runningAgents = useMemo(
    () => agents.filter((a) => a.status !== "stopped" && a.status !== "killed"),
    [agents]
  );

  const totalTrades = trades.length;

  const tradesLast24h = useMemo(() => {
    const cutoff = Date.now() - 24 * 60 * 60 * 1000;
    return trades.filter((t) => new Date(t.time).getTime() >= cutoff).length;
  }, [trades]);

  const totalPnlBase = useMemo(
    () => agents.reduce((sum, a) => sum + (a.totalPnLBase || 0), 0),
    [agents]
  );

  const pnlIsProfit = totalPnlBase >= 0;

  const pnlPctText = "—";

  useEffect(() => {
    if (tradesAutoScroll && tradesScrollRef.current) {
      tradesScrollRef.current.scrollTop = tradesScrollRef.current.scrollHeight;
    }
  }, [trades, tradesAutoScroll]);

  const hasLiveAgent = useMemo(
    () => agents.some((a) => (a.mode || "").toLowerCase() === "live"),
    [agents]
  );

  // Light “demo user” header chip (UI only) when no wallet
  const demoUser = {
    name: "Demo User",
    wallet: "sol_wallet…7x9k",
    avatarUrl:
      "https://lh3.googleusercontent.com/aida-public/AB6AXuADAm2lpytuV_3kDPtuPoLg0ussXeLyw5BkuGQjaowMNBY9PjCrMCjGjzb4l-rK3qnMJoGLC62-oLEMatCYegWNdfuRkTWfiSbWyslYtgIb74J_UT4CJOTuTwET1FGQXPofyZFbSa6B1NLMUrmuvHDzEfMbh-S62xUPJbuhSLZS9nB6ThDgq0x38NpkQFdbrFHTDCOF3dl2dNqHUyeIdSHzfet6ijp650Sr3L1B-5wjiypMxMDoXVRkEpxlK_clrLkFGnny-kygQMhY",
  };

  return (
    <div className="min-h-screen bg-background-light dark:bg-background-dark text-slate-900 dark:text-slate-100">
      <div className="max-w-[1440px] mx-auto px-4 sm:px-6 lg:px-8">
        {/* Live warning banner */}
        {hasLiveAgent && (
          <div className="bg-amber-500/20 border border-amber-500/50 text-amber-200 px-4 py-2 flex items-center gap-2 rounded-lg mb-4">
            <span className="material-symbols-outlined">warning</span>
            <span className="text-sm font-bold uppercase tracking-wider">
              Real trades enabled — Live mode agents are active
            </span>
          </div>
        )}

        {/* Header */}
        <header className="flex items-center justify-between py-6 border-b border-border-dark">
          <div className="flex items-center gap-3">
            <div className="bg-primary p-1.5 rounded-lg">
              <span className="material-symbols-outlined text-white text-2xl">
                monitoring
              </span>
            </div>
            <div>
              <h1 className="text-xl font-bold tracking-tight">
                TradeBot{" "}
                <span className="text-slate-500 font-medium">– Phase 2</span>
              </h1>
              <p className="text-xs text-slate-400 uppercase tracking-widest font-semibold">
                {hasLiveAgent ? "Live + Paper" : "Paper Trading Mode"}
              </p>
            </div>
          </div>

          <div className="flex items-center gap-4">
            {/* Status pill */}
            <div className="flex items-center gap-2 px-3 py-1.5 bg-profit/10 border border-profit/20 rounded-full">
              <span className="relative flex h-2 w-2">
                <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-profit opacity-75"></span>
                <span className="relative inline-flex rounded-full h-2 w-2 bg-profit"></span>
              </span>
              <span className="text-profit text-xs font-bold uppercase tracking-wider">
                Bot Running
              </span>
            </div>

            <div className="h-8 w-[1px] bg-border-dark mx-2 hidden sm:block" />

            {/* Wallet or not configured hint */}
            <div className="flex items-center gap-3">
              {wallet?.pubkey ? (
                <div className="text-right">
                  <p className="text-[10px] text-slate-500 uppercase tracking-wider">
                    Wallet
                  </p>
                  <p className="text-xs font-mono">
                    {shortMint(wallet.pubkey, 4, 4)}
                  </p>
                  <p className="text-xs text-slate-400">
                    {wallet.solBalance.toFixed(4)} SOL
                  </p>
                </div>
              ) : (
                <div className="text-right max-w-[200px]">
                  <p className="text-[10px] text-slate-500 uppercase tracking-wider">
                    Wallet
                  </p>
                  <p className="text-xs text-amber-400/90">
                    Not configured
                  </p>
                  <p className="text-[10px] text-slate-500 mt-0.5">
                    Set WALLET_KEYPAIR_PATH or WALLET_KEYPAIR_BASE64 in .env and restart the server.
                  </p>
                </div>
              )}
            </div>
          </div>
        </header>

        {/* Summary Bar */}
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4 py-6">
          <div className="bg-card-dark border border-border-dark rounded-xl p-5 flex items-center gap-4">
            <div className="p-3 bg-primary/10 rounded-lg">
              <span className="material-symbols-outlined text-primary">
                smart_toy
              </span>
            </div>
            <div>
              <p className="text-slate-400 text-sm font-medium">
                Total Agents Running
              </p>
              <p className="text-2xl font-bold">
                {runningAgents.length}{" "}
                <span className="text-xs text-slate-500 font-normal ml-2">
                  Active
                </span>
              </p>
            </div>
          </div>

          <div className="bg-card-dark border border-border-dark rounded-xl p-5 flex items-center gap-4">
            <div className="p-3 bg-slate-800 rounded-lg">
              <span className="material-symbols-outlined text-slate-400">
                swap_horiz
              </span>
            </div>
            <div>
              <p className="text-slate-400 text-sm font-medium">
                Total Trades (24h)
              </p>
              <p className="text-2xl font-bold">
                {tradesLast24h}{" "}
                <span className="text-xs text-slate-500 font-medium ml-2">
                  of {totalTrades} total
                </span>
              </p>
            </div>
          </div>

          <div className="bg-card-dark border border-border-dark rounded-xl p-5 flex items-center gap-4">
            <div className={cx("p-3 rounded-lg", pnlIsProfit ? "bg-profit/10" : "bg-loss/10")}>
              <span
                className={cx(
                  "material-symbols-outlined",
                  pnlIsProfit ? "text-profit" : "text-loss"
                )}
              >
                payments
              </span>
            </div>
            <div>
              <p className="text-slate-400 text-sm font-medium">Total PnL</p>
              <p
                className={cx(
                  "text-2xl font-bold",
                  pnlIsProfit ? "text-profit" : "text-loss"
                )}
              >
                {formatSigned(totalPnlBase, 6)}{" "}
                <span className="text-xs font-medium ml-2">SOL</span>{" "}
                <span className="text-xs text-slate-500 font-medium ml-2">
                  {pnlPctText}
                </span>
              </p>
            </div>
          </div>
        </div>

        {/* Main Content Grid */}
        <div className="grid grid-cols-1 lg:grid-cols-12 gap-6 pb-12">
          {/* Left Column (5/12) */}
          <div className="lg:col-span-5 flex flex-col gap-6">
            {/* Create Agent */}
            <section className="bg-card-dark border border-border-dark rounded-xl overflow-hidden">
              <div className="p-5 border-b border-border-dark bg-slate-900/50">
                <h2 className="font-bold flex items-center gap-2">
                  <span className="material-symbols-outlined text-primary">
                    add_circle
                  </span>
                  Create Agent
                </h2>
              </div>

              <div className="p-6">
                <CreateAgentForm
                  onCreated={() => {
                    void refreshAgents();
                    void refreshTrades();
                  }}
                />
              </div>
            </section>
          </div>

          {/* Right Column (7/12) */}
          <div className="lg:col-span-7 flex flex-col gap-6">
            {/* Active Agents */}
            <section className="bg-card-dark border border-border-dark rounded-xl overflow-hidden flex flex-col max-h-[420px]">
              <div className="p-5 border-b border-border-dark bg-slate-900/50 flex justify-between items-center shrink-0">
                <h2 className="font-bold flex items-center gap-2">
                  <span className="material-symbols-outlined text-primary">
                    groups
                  </span>
                  Active Agents
                </h2>
                <span className="text-xs bg-slate-800 text-slate-400 px-2 py-1 rounded">
                  {runningAgents.length} Running
                </span>
              </div>

              <div className="p-6 overflow-y-auto flex-1 min-h-0 custom-scrollbar">
                <AgentsPanel
                  agents={agents}
                  trades={trades}
                  onChanged={() => {
                    void refreshAgents();
                    void refreshTrades();
                  }}
                />
              </div>
            </section>

            {/* Trades history */}
            <section className="bg-card-dark border border-border-dark rounded-xl overflow-hidden flex flex-col h-[420px]">
              <div className="p-5 border-b border-border-dark bg-slate-900/50 flex justify-between items-center flex-wrap gap-2">
                <h2 className="font-bold flex items-center gap-2">
                  <span className="material-symbols-outlined text-primary">
                    history
                  </span>
                  Trades History
                </h2>

                <div className="flex items-center gap-4">
                  <label className="flex items-center gap-2 cursor-pointer select-none">
                    <input
                      type="checkbox"
                      checked={tradesAutoScroll}
                      onChange={(e) => setTradesAutoScroll(e.target.checked)}
                      className="rounded border-slate-700 bg-slate-800 text-primary focus:ring-0 focus:ring-offset-0"
                      aria-label="Auto-scroll trades"
                    />
                    <span className="text-[10px] font-bold uppercase tracking-wider text-slate-400">
                      Auto-scroll
                    </span>
                  </label>
                  <a
                    href="/api/trades.csv"
                    download="trades.csv"
                    className="text-xs text-primary font-bold hover:underline"
                  >
                    Download CSV
                  </a>
                </div>
              </div>

              {/* Scroll container */}
              <div
                ref={tradesScrollRef}
                className="flex-1 overflow-y-auto custom-scrollbar min-h-0"
              >
                <div className="min-w-full p-4">
                  <TradesPanel trades={trades} />
                </div>
              </div>
            </section>

            {/* Optional: Pair chips/legend (UI only). Keep minimal. */}
            {runningAgents.length > 0 && (
              <div className="hidden">
                {runningAgents.map((a) => (
                  <span key={a.id}>
                    {shortMint(a.baseMint)}→{shortMint(a.targetMint)}
                  </span>
                ))}
              </div>
            )}
          </div>
        </div>

        {/* Live Logs (full width) */}
        <section className="mt-6 bg-[#0b0f1a] border border-border-dark rounded-xl overflow-hidden flex flex-col min-h-[260px] lg:min-h-[320px]">
          <LogsPanel />
          <p className="px-4 py-2 text-[10px] text-slate-500 border-t border-border-dark bg-slate-900/30">
            Paper positions are cleared on restart.
          </p>
        </section>
      </div>
    </div>
  );
};