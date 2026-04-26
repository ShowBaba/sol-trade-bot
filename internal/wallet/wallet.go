package wallet

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"crypto/ed25519"

	"github.com/mr-tron/base58"
)

// Wallet loads a keypair from env and exposes Pubkey and Sign.
// No secrets are ever logged.
type Wallet struct {
	privateKey ed25519.PrivateKey // 64 bytes: seed (32) + pub (32)
	publicKey  []byte             // 32 bytes, cached
}

// LoadFromEnv loads keypair from WALLET_KEYPAIR_PATH (JSON file) or
// WALLET_KEYPAIR_BASE64 (base64-encoded array). Path takes precedence if both set.
func LoadFromEnv() (*Wallet, error) {
	path := os.Getenv("WALLET_KEYPAIR_PATH")
	if path != "" {
		return LoadFromFile(path)
	}
	b64 := os.Getenv("WALLET_KEYPAIR_BASE64")
	if b64 != "" {
		return LoadFromBase64(b64)
	}
	return nil, errors.New("no wallet keypair: set WALLET_KEYPAIR_PATH or WALLET_KEYPAIR_BASE64")
}

// LoadFromFile reads a Solana keypair JSON file (array of 64 bytes).
func LoadFromFile(path string) (*Wallet, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var arr []byte
	if err := json.Unmarshal(data, &arr); err != nil {
		return nil, err
	}
	if len(arr) != ed25519.PrivateKeySize {
		return nil, errors.New("keypair must be 64 bytes")
	}
	return &Wallet{
		privateKey: ed25519.PrivateKey(arr),
		publicKey:  arr[32:64],
	}, nil
}

// LoadFromBase64 decodes a base64-encoded keypair. Accepts either:
//   - Base64 of 64 raw bytes (standard), or
//   - Base64 of a JSON array of 64 numbers (e.g. base64("[191,118,...]") from Phantom export).
func LoadFromBase64(b64 string) (*Wallet, error) {
	decoded, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, err
	}
	arr := decoded
	if len(arr) != ed25519.PrivateKeySize {
		// Try parsing as JSON array (e.g. base64-encoded "[191,118,...]")
		if len(decoded) > 2 && decoded[0] == '[' {
			var jsonArr []byte
			if err := json.Unmarshal(decoded, &jsonArr); err != nil {
				return nil, errors.New("keypair must be 64 bytes or base64 of JSON array of 64 numbers")
			}
			arr = jsonArr
		}
	}
	if len(arr) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("keypair must be 64 bytes, got %d", len(arr))
	}
	return &Wallet{
		privateKey: ed25519.PrivateKey(arr),
		publicKey:  arr[32:64],
	}, nil
}

// PublicKeyBase58 returns the public key in Solana base58 encoding.
// Caller must not log the full key if avoiding PII; we never log private key.
func (w *Wallet) PublicKeyBase58() string {
	return base58.Encode(w.publicKey)
}

// PublicKeyBytes returns the 32-byte public key (do not log).
func (w *Wallet) PublicKeyBytes() []byte {
	return w.publicKey
}

// SignMessage signs the message with ed25519 and returns the 64-byte signature.
func (w *Wallet) SignMessage(message []byte) ([]byte, error) {
	return ed25519.Sign(w.privateKey, message), nil
}

// SignTransaction signs a serialized Solana transaction (wire format).
// Layout: compact-u16 numSignatures, then 64 bytes per signature, then message.
// We sign the message and write the signature into the first slot.
func (w *Wallet) SignTransaction(txBytes []byte) ([]byte, error) {
	if len(txBytes) < 2 {
		return nil, errors.New("tx too short")
	}
	nSig, off := decodeCompactU16(txBytes)
	if off <= 0 || nSig == 0 {
		return nil, errors.New("invalid tx signature count")
	}
	sigLen := int(nSig) * 64
	if len(txBytes) < off+sigLen+1 {
		return nil, errors.New("tx too short for signatures and message")
	}
	message := txBytes[off+sigLen:]
	sig := ed25519.Sign(w.privateKey, message)
	out := make([]byte, len(txBytes))
	copy(out, txBytes)
	copy(out[off:], sig)
	return out, nil
}

// decodeCompactU16 decodes Solana compact-u16; returns value and bytes consumed (0 if invalid).
func decodeCompactU16(b []byte) (uint16, int) {
	if len(b) < 1 {
		return 0, 0
	}
	v := uint16(b[0])
	if v < 127 {
		return v, 1
	}
	if len(b) < 2 {
		return 0, 0
	}
	v = (v & 0x7f) | uint16(b[1])<<7
	if b[1] < 128 {
		return v, 2
	}
	if len(b) < 3 {
		return 0, 0
	}
	v |= uint16(b[2]) << 14
	return v, 3
}
