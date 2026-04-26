package risk

import (
	"errors"
	"fmt"
)

// MintInfo is used for optional token safety checks (authority flags, decimals).
type MintInfo struct {
	HasMintAuthority   bool
	HasFreezeAuthority bool
	Decimals           uint8
}

// PreTradeChecks runs optional pre-trade checks. Call before exec.Enter in live mode.
// - allowedMints: if non-empty, baseMint and targetMint must be in list.
// - deniedMints: if non-empty, baseMint and targetMint must not be in list.
// - baseMintInfo/targetMintInfo: if non-nil, reject if HasMintAuthority or HasFreezeAuthority (optional token safety).
func PreTradeChecks(baseMint, targetMint string, allowedMints, deniedMints []string, baseMintInfo, targetMintInfo *MintInfo) error {
	if len(allowedMints) > 0 {
		if !inList(baseMint, allowedMints) || !inList(targetMint, allowedMints) {
			return errors.New("mint not in allowlist")
		}
	}
	if len(deniedMints) > 0 {
		if inList(baseMint, deniedMints) || inList(targetMint, deniedMints) {
			return errors.New("mint is in denylist")
		}
	}
	if baseMintInfo != nil {
		if baseMintInfo.HasMintAuthority {
			return fmt.Errorf("base mint has mint authority")
		}
		if baseMintInfo.HasFreezeAuthority {
			return fmt.Errorf("base mint has freeze authority")
		}
		if baseMintInfo.Decimals > 18 {
			return fmt.Errorf("base mint decimals sanity check failed")
		}
	}
	if targetMintInfo != nil {
		if targetMintInfo.HasMintAuthority {
			return fmt.Errorf("target mint has mint authority")
		}
		if targetMintInfo.HasFreezeAuthority {
			return fmt.Errorf("target mint has freeze authority")
		}
		if targetMintInfo.Decimals > 18 {
			return fmt.Errorf("target mint decimals sanity check failed")
		}
	}
	return nil
}

func inList(mint string, list []string) bool {
	for _, m := range list {
		if m == mint {
			return true
		}
	}
	return false
}
