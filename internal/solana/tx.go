package solana

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// SendRawTransaction sends a signed transaction and returns the signature.
func (c *RPCClient) SendRawTransaction(ctx context.Context, tx []byte) (sig string, err error) {
	txB64 := base64.StdEncoding.EncodeToString(tx)
	reqBody := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "sendTransaction",
		"params": []any{
			txB64,
			map[string]any{
				"encoding":            "base64",
				"skipPreflight":       false,
				"preflightCommitment": "processed",
				"maxRetries":          3,
			},
		},
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(reqBody); err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, &buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("solana rpc http %d", resp.StatusCode)
	}
	var rpcResp sendTxResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return "", err
	}
	if rpcResp.Error != nil {
		msg := rpcResp.Error.Message
		// Map common program errors to a clearer message (0x1 = SPL Token InsufficientFunds)
		if strings.Contains(msg, "custom program error: 0x1") {
			msg = msg + " (InsufficientFunds: not enough SOL for trade+fees+rent, or not enough source token balance)"
		}
		return "", fmt.Errorf("sendTransaction: %s", msg)
	}
	if rpcResp.Result == "" {
		return "", fmt.Errorf("sendTransaction: empty result")
	}
	return rpcResp.Result, nil
}

type sendTxResponse struct {
	Result string `json:"result"`
	Error  *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// ConfirmTransaction polls getSignatureStatuses until the tx is confirmed or timeout.
func (c *RPCClient) ConfirmTransaction(ctx context.Context, sig string, commitment string, timeout time.Duration) error {
	if commitment == "" {
		commitment = "confirmed"
	}
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				return fmt.Errorf("confirmation timeout after %v", timeout)
			}
			reqBody := map[string]any{
				"jsonrpc": "2.0",
				"id":      1,
				"method":  "getSignatureStatuses",
				"params":  []any{[]string{sig}},
			}
			var buf bytes.Buffer
			_ = json.NewEncoder(&buf).Encode(reqBody)
			req, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, &buf)
			req.Header.Set("Content-Type", "application/json")
			resp, err := c.http.Do(req)
			if err != nil {
				continue
			}
			var statusResp getSignatureStatusesResponse
			_ = json.NewDecoder(resp.Body).Decode(&statusResp)
			resp.Body.Close()
			if statusResp.Result != nil && statusResp.Result.Value != nil && len(statusResp.Result.Value) > 0 {
				st := statusResp.Result.Value[0]
				if st != nil && st.ConfirmationStatus != nil {
					switch *st.ConfirmationStatus {
					case "confirmed", "finalized":
						return nil
					case "processed":
						if commitment == "processed" {
							return nil
						}
					}
					if st.Err != nil {
						return fmt.Errorf("transaction failed: %v", st.Err)
					}
				}
			}
		}
	}
}

type getSignatureStatusesResponse struct {
	Result *struct {
		Value []*struct {
			ConfirmationStatus *string     `json:"confirmationStatus"`
			Err                interface{} `json:"err"`
		} `json:"value"`
	} `json:"result"`
}
