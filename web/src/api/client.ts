import type { AgentSnapshot, Trade, LogEvent } from "../types";

async function handleJson<T>(res: Response): Promise<T> {
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || res.statusText);
  }
  return (await res.json()) as T;
}

export async function fetchAgents(): Promise<AgentSnapshot[]> {
  const res = await fetch("/api/agents");
  return handleJson<AgentSnapshot[]>(res);
}

export interface CreateAgentPayload {
  id?: string;
  baseMint: string;
  targetMint: string;
  mode?: "paper" | "live";
  tradeSizeLamports: number;
  pollSeconds: number;
  cooldownSeconds: number;
  maxSlippageBps: number;
  takeProfitPct: number;
  stopLossPct: number;
  maxConsecutiveLosses: number;
  maxTotalLossSOL: number;
  maxPriceImpactPct: number;
  minOutTargetSmallest?: number;
  quoteStabilityWindow?: number;
  maxJitterPct?: number;
}

export interface WalletInfo {
  pubkey: string;
  solBalance: number;
  configured?: boolean;
}

export async function fetchWallet(): Promise<WalletInfo> {
  const res = await fetch("/api/wallet");
  return handleJson<WalletInfo>(res);
}

export async function createAgent(payload: CreateAgentPayload): Promise<AgentSnapshot> {
  const res = await fetch("/api/agents/create", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload)
  });
  return handleJson<AgentSnapshot>(res);
}

export async function startAgent(id: string): Promise<void> {
  const res = await fetch(`/api/agents/start?id=${encodeURIComponent(id)}`, { method: "POST" });
  await handleJson<Record<string, unknown>>(res);
}

export async function stopAgent(id: string): Promise<void> {
  const res = await fetch(`/api/agents/stop?id=${encodeURIComponent(id)}`, { method: "POST" });
  await handleJson<Record<string, unknown>>(res);
}

export async function deleteAgent(id: string): Promise<void> {
  const res = await fetch(`/api/agents/delete?id=${encodeURIComponent(id)}`, { method: "POST" });
  await handleJson<Record<string, unknown>>(res);
}

export async function resetAgent(id: string): Promise<void> {
  const res = await fetch(`/api/agents/reset?id=${encodeURIComponent(id)}`, { method: "POST" });
  await handleJson<Record<string, unknown>>(res);
}

export async function fetchTrades(): Promise<Trade[]> {
  const res = await fetch("/api/trades");
  return handleJson<Trade[]>(res);
}

export async function fetchLogHistory(): Promise<LogEvent[]> {
  const res = await fetch("/api/logs/history");
  return handleJson<LogEvent[]>(res);
}

export function createLogsEventSource(onMessage: (e: LogEvent) => void, onError?: (err: any) => void) {
  const es = new EventSource("/api/logs");
  es.onmessage = (ev) => {
    try {
      const parsed = JSON.parse(ev.data) as LogEvent;
      onMessage(parsed);
    } catch (err) {
      if (onError) onError(err);
    }
  };
  es.onerror = (err) => {
    if (onError) onError(err);
  };
  return es;
}

export async function clearLogs(): Promise<void> {
  const res = await fetch("/api/logs/clear", { method: "POST" });
  await handleJson<Record<string, unknown>>(res);
}

