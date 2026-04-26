export type AgentStatus =
  | "stopped"
  | "waiting"
  | "in_position"
  | "cooldown"
  | "killed"
  | "needs_manual_attention";

export interface AgentPosition {
  entryTime: string;
  entryBase: number;
  entryPriceBasePerT: number;
  targetQty: number;
  entryTxSig?: string;
}

export interface AgentSnapshot {
  id: string;
  baseMint: string;
  targetMint: string;
  status: AgentStatus;
  mode: string;
  consecutiveLosses: number;
  totalPnLBase: number;
  tradeSizeBase?: number;
  tradeSizeBaseSmallestDisplay?: number;
  takeProfitPct?: number;
  stopLossPct?: number;
  position?: AgentPosition;
  lastError?: string;
}

export interface Trade {
  time: string;
  agentID: string;
  action: "enter" | "exit";
  baseMint: string;
  targetMint: string;
  entryBaseSmallest: number;
  targetQtySmallest: number;
  exitBaseSmallest: number;
  pnlBaseSmallest: number;
  pnlPct: number;
  reason: string;
  entryTxSig?: string;
  exitTxSig?: string;
  feesLamports?: number;
}

export interface LogEvent {
  time: string;
  level: string;
  agent: string;
  msg: string;
  fields?: unknown;
}

