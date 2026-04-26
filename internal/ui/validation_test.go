package ui

import (
	"testing"
)

func TestValidateCreateReq_TP_SL(t *testing.T) {
	validReq := createReq{
		BaseMint:       "So11111111111111111111111111111111111111112",
		TargetMint:     "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
		TakeProfitPct:  5,
		StopLossPct:    2,
		PollSeconds:     5,
		CooldownSeconds: 60,
		MaxSlippageBps: 100,
	}
	tests := []struct {
		name    string
		modify  func(*createReq)
		wantErr bool
	}{
		{"valid", nil, false},
		{"TP negative", func(r *createReq) { r.TakeProfitPct = -1 }, true},
		{"TP over 100", func(r *createReq) { r.TakeProfitPct = 101 }, true},
		{"TP 100 ok", func(r *createReq) { r.TakeProfitPct = 100 }, false},
		{"SL negative", func(r *createReq) { r.StopLossPct = -0.5 }, true},
		{"SL over 100", func(r *createReq) { r.StopLossPct = 100.1 }, true},
		{"SL 0 ok", func(r *createReq) { r.StopLossPct = 0 }, false},
		{"missing baseMint", func(r *createReq) { r.BaseMint = "" }, true},
		{"missing targetMint", func(r *createReq) { r.TargetMint = "" }, true},
		{"mode paper ok", func(r *createReq) { r.Mode = "paper" }, false},
		{"mode live ok", func(r *createReq) { r.Mode = "live" }, false},
		{"mode invalid", func(r *createReq) { r.Mode = "invalid" }, true},
		{"mode empty ok", func(r *createReq) { r.Mode = "" }, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := validReq
			if tt.modify != nil {
				tt.modify(&req)
			}
			err := validateCreateReq(req)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateCreateReq() err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
