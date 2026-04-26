package ui

import (
	"encoding/csv"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ShowBaba/tradebot/internal/agent"
)

func TestTradesCSVFormat(t *testing.T) {
	header := []string{"time", "agentID", "action", "baseMint", "targetMint", "entryBaseSmallest", "targetQtySmallest", "exitBaseSmallest", "pnlBaseSmallest", "pnlPct", "reason", "entryTxSig", "exitTxSig", "feesLamports"}
	trade := agent.Trade{
		Time:               time.Date(2025, 3, 4, 12, 0, 0, 0, time.UTC),
		AgentID:            "sol-war-1",
		Action:             "exit",
		BaseMint:           "So11111111111111111111111111111111111111112",
		TargetMint:         "WAR",
		EntryBaseSmallest:  100_000_000,
		TargetQtySmallest:  50_000,
		ExitBaseSmallest:   105_000_000,
		PnlBaseSmallest:    5_000_000,
		PnlPct:             5.0,
		Reason:             "take_profit",
		EntryTxSig:         "entrySig123",
		ExitTxSig:          "exitSig456",
		FeesLamports:       5000,
	}
	row := []string{
		trade.Time.Format(time.RFC3339),
		trade.AgentID,
		trade.Action,
		trade.BaseMint,
		trade.TargetMint,
		strconv.FormatUint(trade.EntryBaseSmallest, 10),
		strconv.FormatUint(trade.TargetQtySmallest, 10),
		strconv.FormatUint(trade.ExitBaseSmallest, 10),
		strconv.FormatInt(trade.PnlBaseSmallest, 10),
		strconv.FormatFloat(trade.PnlPct, 'f', 4, 64),
		trade.Reason,
		trade.EntryTxSig,
		trade.ExitTxSig,
		strconv.FormatUint(trade.FeesLamports, 10),
	}
	if len(row) != len(header) {
		t.Fatalf("row has %d fields, header has %d", len(row), len(header))
	}
	var buf strings.Builder
	wr := csv.NewWriter(&buf)
	_ = wr.Write(header)
	_ = wr.Write(row)
	wr.Flush()
	out := buf.String()
	if !strings.Contains(out, "time,") {
		t.Errorf("CSV missing header: %q", out)
	}
	if !strings.Contains(out, "take_profit") {
		t.Errorf("CSV missing reason: %q", out)
	}
}
