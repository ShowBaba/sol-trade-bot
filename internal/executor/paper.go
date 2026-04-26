package executor

import (
	"context"

	"github.com/ShowBaba/tradebot/internal/jupiter"
)

// PaperExecutor executes entry/exit in memory only (no tx, no network).
type PaperExecutor struct{}

// NewPaper returns a paper executor.
func NewPaper() *PaperExecutor {
	return &PaperExecutor{}
}

// Enter returns expected out from the quote; no transaction.
func (e *PaperExecutor) Enter(ctx context.Context, cfg Config, quote *jupiter.QuoteResponse) ExecutionResult {
	out, err := jupiter.ParseAmountUint(quote.OutAmount)
	if err != nil || out == 0 {
		return ExecutionResult{Error: err}
	}
	return ExecutionResult{ExpectedOut: out}
}

// Exit returns expected base out from the exit quote; no transaction.
func (e *PaperExecutor) Exit(ctx context.Context, cfg Config, quote *jupiter.QuoteResponse, pos *Position) ExecutionResult {
	out, err := jupiter.ParseAmountUint(quote.OutAmount)
	if err != nil || out == 0 {
		return ExecutionResult{Error: err}
	}
	return ExecutionResult{ExpectedOut: out}
}
