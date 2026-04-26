package executor

import (
	"context"
	"time"

	"github.com/ShowBaba/tradebot/internal/jupiter"
)

// ExecutionResult is the result of an Enter or Exit execution.
type ExecutionResult struct {
	TxSignature  string
	ExpectedOut  uint64
	ActualOut    uint64
	FeesLamports uint64
	SlippageBps  int
	Error        error
}

// Config is the minimal config passed from agent to executor for a single trade.
type Config struct {
	BaseMint               string
	TargetMint             string
	BaseDecimals           uint8
	TargetDecimals         uint8
	TradeSizeBaseSmallest  uint64
	MaxSlippageBps         int
}

// Position is the minimal position info for exit (avoids importing agent).
type Position struct {
	EntryTime          time.Time
	EntryBaseSmallest  uint64
	TargetQtySmallest  uint64
}

// Executor executes entry and exit (paper or live).
type Executor interface {
	Enter(ctx context.Context, cfg Config, quote *jupiter.QuoteResponse) ExecutionResult
	Exit(ctx context.Context, cfg Config, quote *jupiter.QuoteResponse, pos *Position) ExecutionResult
}

// Signer is implemented by wallet for live execution. Defined here to avoid executor depending on wallet.
type Signer interface {
	PublicKeyBase58() string
	SignTransaction(tx []byte) ([]byte, error)
}

// LiveConfig configures live executor (retries, confirmation, dry-run).
type LiveConfig struct {
	PriorityFeeMicroLamports int           // 0 = use Jupiter default
	ComputeUnits             int           // 0 = use Jupiter default
	MaxRetries               int           // default 3
	ConfirmTimeout          time.Duration // default 60s
	Commitment               string        // "confirmed" or "finalized"
	DryRun                   bool          // build+sign but do not send
}

// TxSender sends and confirms transactions (implemented by solana.RPCClient).
type TxSender interface {
	SendRawTransaction(ctx context.Context, tx []byte) (sig string, err error)
	ConfirmTransaction(ctx context.Context, sig string, commitment string, timeout time.Duration) error
}
