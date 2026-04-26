package executor

import (
	"context"
	"testing"

	"github.com/ShowBaba/tradebot/internal/jupiter"
)

func TestPaperExecutor_Enter(t *testing.T) {
	ctx := context.Background()
	exec := NewPaper()
	cfg := Config{TradeSizeBaseSmallest: 100_000_000}
	quote := &jupiter.QuoteResponse{OutAmount: "50000"}

	res := exec.Enter(ctx, cfg, quote)
	if res.Error != nil {
		t.Fatalf("Enter err = %v", res.Error)
	}
	if res.ExpectedOut != 50000 {
		t.Fatalf("ExpectedOut = %d, want 50000", res.ExpectedOut)
	}
}

func TestPaperExecutor_Enter_InvalidQuote(t *testing.T) {
	ctx := context.Background()
	exec := NewPaper()
	cfg := Config{}
	quote := &jupiter.QuoteResponse{OutAmount: "invalid"}

	res := exec.Enter(ctx, cfg, quote)
	if res.Error == nil {
		t.Fatal("Enter expected error for invalid OutAmount")
	}
}

func TestPaperExecutor_Exit(t *testing.T) {
	ctx := context.Background()
	exec := NewPaper()
	cfg := Config{}
	quote := &jupiter.QuoteResponse{OutAmount: "105000000"}
	pos := &Position{TargetQtySmallest: 50000}

	res := exec.Exit(ctx, cfg, quote, pos)
	if res.Error != nil {
		t.Fatalf("Exit err = %v", res.Error)
	}
	if res.ExpectedOut != 105000000 {
		t.Fatalf("ExpectedOut = %d, want 105000000", res.ExpectedOut)
	}
}
