package token

import (
	"context"
	"math"

	"github.com/ShowBaba/tradebot/internal/solana"
)

// Service wraps a Solana RPC client and exposes token decimal helpers.
type Service struct {
	rpc *solana.RPCClient
}

func NewService(rpc *solana.RPCClient) *Service {
	return &Service{rpc: rpc}
}

// Decimals returns the decimals for a given mint.
func (s *Service) Decimals(ctx context.Context, mint string) (uint8, error) {
	return s.rpc.GetMintDecimals(ctx, mint)
}

// SmallestToFloat converts an amount in smallest units (e.g. lamports) to a
// human float using the given decimals.
func SmallestToFloat(amount uint64, decimals uint8) float64 {
	if decimals == 0 {
		return float64(amount)
	}
	scale := math.Pow10(int(decimals))
	return float64(amount) / scale
}

// FloatToSmallest converts a human float amount into smallest units using the
// given decimals, rounding to the nearest integer.
func FloatToSmallest(amount float64, decimals uint8) uint64 {
	if decimals == 0 {
		if amount < 0 {
			return 0
		}
		return uint64(math.Round(amount))
	}
	scale := math.Pow10(int(decimals))
	v := math.Round(amount * scale)
	if v < 0 {
		return 0
	}
	return uint64(v)
}

// MintSafetyResult holds mint authority and decimals for safety checks.
type MintSafetyResult struct {
	HasMintAuthority   bool
	HasFreezeAuthority bool
	Decimals           uint8
}

// MintSafety fetches mint account and returns authority flags and decimals.
func (s *Service) MintSafety(ctx context.Context, mint string) (MintSafetyResult, error) {
	info, err := s.rpc.GetMintAccountInfo(ctx, mint)
	if err != nil {
		return MintSafetyResult{}, err
	}
	return MintSafetyResult{
		HasMintAuthority:   info.HasMintAuthority,
		HasFreezeAuthority: info.HasFreezeAuthority,
		Decimals:           info.Decimals,
	}, nil
}
