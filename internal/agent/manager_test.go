package agent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ShowBaba/tradebot/internal/jupiter"
	"github.com/ShowBaba/tradebot/internal/logbus"
)

func TestManager_ExecForMode_Selection(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	bus := logbus.New(100)
	jup := jupiter.New()
	mgr := NewManager(jup, bus, &FileBackend{Path: statePath})
	defer func() { _ = os.RemoveAll(dir) }()

	// No wallet: both ModePaper and ModeLive should create agents (Live falls back to Paper).
	cfgPaper := Config{
		ID:                    "paper-1",
		BaseMint:              "So11111111111111111111111111111111111111112",
		TargetMint:            "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
		Mode:                  ModePaper,
		TradeSizeBaseSmallest: 100_000_000,
		PollInterval:          0,
		TakeProfitPct:         5,
		StopLossPct:           2,
	}
	_, err := mgr.Create(cfgPaper)
	if err != nil {
		t.Fatalf("Create(ModePaper) err = %v", err)
	}

	cfgLive := Config{
		ID:                    "live-1",
		BaseMint:              "So11111111111111111111111111111111111111112",
		TargetMint:            "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
		Mode:                  ModeLive,
		TradeSizeBaseSmallest: 100_000_000,
		PollInterval:          0,
		TakeProfitPct:         5,
		StopLossPct:           2,
	}
	_, err = mgr.Create(cfgLive)
	if err != nil {
		t.Fatalf("Create(ModeLive) without wallet err = %v (should fallback to Paper)", err)
	}

	list := mgr.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(list))
	}
}
