package executor

import (
	"context"
	"encoding/base64"
	"errors"
	"log"
	"time"

	"github.com/ShowBaba/tradebot/internal/jupiter"
)

// LiveExecutor executes entry/exit via Jupiter swap API and Solana RPC.
type LiveExecutor struct {
	jup    *jupiter.Client
	signer Signer
	rpc    TxSender
	cfg    LiveConfig
}

// NewLive returns a live executor.
func NewLive(jup *jupiter.Client, signer Signer, rpc TxSender, cfg LiveConfig) *LiveExecutor {
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}
	if cfg.ConfirmTimeout <= 0 {
		cfg.ConfirmTimeout = 60 * time.Second
	}
	if cfg.Commitment == "" {
		cfg.Commitment = "confirmed"
	}
	return &LiveExecutor{jup: jup, signer: signer, rpc: rpc, cfg: cfg}
}

// Enter builds a swap tx from the quote, signs, sends (unless DryRun), and confirms.
func (e *LiveExecutor) Enter(ctx context.Context, cfg Config, quote *jupiter.QuoteResponse) ExecutionResult {
	return e.swap(ctx, quote, cfg.TradeSizeBaseSmallest, quote.OutAmount, "enter")
}

// Exit builds a swap tx (target->base) from the quote, signs, sends, and confirms.
func (e *LiveExecutor) Exit(ctx context.Context, cfg Config, quote *jupiter.QuoteResponse, pos *Position) ExecutionResult {
	expectedOut := quote.OutAmount
	return e.swap(ctx, quote, pos.TargetQtySmallest, expectedOut, "exit")
}

func (e *LiveExecutor) swap(ctx context.Context, quote *jupiter.QuoteResponse, amount uint64, expectedOutStr string, _ string) ExecutionResult {
	expectedOut, _ := jupiter.ParseAmountUint(expectedOutStr)
	userPubkey := e.signer.PublicKeyBase58()
	sr, err := e.jup.Swap(ctx, quote, userPubkey, nil)
	if err != nil {
		return ExecutionResult{Error: err, ExpectedOut: expectedOut}
	}
	txBytes, err := base64.StdEncoding.DecodeString(sr.SwapTransaction)
	if err != nil {
		return ExecutionResult{Error: err, ExpectedOut: expectedOut}
	}
	signed, err := e.signer.SignTransaction(txBytes)
	if err != nil {
		return ExecutionResult{Error: err, ExpectedOut: expectedOut}
	}
	if e.cfg.DryRun {
		log.Printf("dry-run: swap tx built and signed but not sent (lastValidBlockHeight=%d)", sr.LastValidBlockHeight)
		return ExecutionResult{
			TxSignature:  "dry-run",
			ExpectedOut:  expectedOut,
			FeesLamports: sr.PrioritizationFeeLamports,
		}
	}
	var sig string
	var lastErr error
	for attempt := 0; attempt <= e.cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt*500) * time.Millisecond)
			// Optionally re-fetch quote and rebuild tx on blockhash expiry; for now retry same tx
		}
		sig, lastErr = e.rpc.SendRawTransaction(ctx, signed)
		if lastErr != nil {
			if ctx.Err() != nil {
				return ExecutionResult{Error: ctx.Err(), ExpectedOut: expectedOut}
			}
			continue
		}
		lastErr = e.rpc.ConfirmTransaction(ctx, sig, e.cfg.Commitment, e.cfg.ConfirmTimeout)
		if lastErr == nil {
			return ExecutionResult{
				TxSignature:  sig,
				ExpectedOut:  expectedOut,
				ActualOut:    expectedOut, // Jupiter doesn't return post-execution amount; use quote
				FeesLamports: sr.PrioritizationFeeLamports,
			}
		}
		if ctx.Err() != nil {
			return ExecutionResult{Error: ctx.Err(), ExpectedOut: expectedOut}
		}
	}
	if lastErr == nil {
		lastErr = errors.New("max retries exceeded")
	}
	return ExecutionResult{Error: lastErr, ExpectedOut: expectedOut}
}
