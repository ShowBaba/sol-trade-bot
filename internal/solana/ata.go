package solana

import (
	"fmt"

	"github.com/gagliardetto/solana-go"
)

// GetAssociatedTokenAddress returns the ATA for (owner, mint) in base58.
func GetAssociatedTokenAddress(ownerBase58, mintBase58 string) (ataBase58 string, err error) {
	owner, err := solana.PublicKeyFromBase58(ownerBase58)
	if err != nil {
		return "", fmt.Errorf("owner pubkey: %w", err)
	}
	mint, err := solana.PublicKeyFromBase58(mintBase58)
	if err != nil {
		return "", fmt.Errorf("mint pubkey: %w", err)
	}
	ata, _, err := solana.FindAssociatedTokenAddress(owner, mint)
	if err != nil {
		return "", err
	}
	return ata.String(), nil
}
