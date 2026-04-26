package solana

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

// RPCClient is a minimal Solana JSON-RPC client focused on SPL mint metadata.
type RPCClient struct {
	endpoint string
	http     *http.Client

	mu       sync.Mutex
	decCache map[string]cachedDecimals
	ttl      time.Duration
}

type cachedDecimals struct {
	value   uint8
	expires time.Time
}

// NewRPCClient creates a new RPC client. Endpoint is taken from SOLANA_RPC_URL
// if empty, defaulting to mainnet-beta.
func NewRPCClient(endpoint string) *RPCClient {
	if endpoint == "" {
		endpoint = os.Getenv("SOLANA_RPC_URL")
	}
	if endpoint == "" {
		log.Fatal("error creating RPC client: SOLANA_RPC_URL not set")
	}
	return &RPCClient{
		endpoint: endpoint,
		http:     &http.Client{Timeout: 10 * time.Second},
		decCache: make(map[string]cachedDecimals),
		ttl:      5 * time.Minute,
	}
}

// GetBalance returns the lamports balance of the account.
func (c *RPCClient) GetBalance(ctx context.Context, pubkeyBase58 string) (lamports uint64, err error) {
	reqBody := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "getBalance",
		"params":  []any{pubkeyBase58},
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(reqBody); err != nil {
		return 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, &buf)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return 0, fmt.Errorf("solana rpc http %d", resp.StatusCode)
	}
	var rpcResp struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return 0, err
	}
	if rpcResp.Error != nil {
		return 0, fmt.Errorf("getBalance: %s", rpcResp.Error.Message)
	}
	if len(rpcResp.Result) == 0 {
		return 0, nil
	}
	// result can be { "context": {...}, "value": N } or just N (lamports)
	var value uint64
	if rpcResp.Result[0] == '{' {
		var obj struct {
			Value uint64 `json:"value"`
		}
		if err := json.Unmarshal(rpcResp.Result, &obj); err != nil {
			return 0, err
		}
		value = obj.Value
	} else {
		if err := json.Unmarshal(rpcResp.Result, &value); err != nil {
			return 0, err
		}
	}
	return value, nil
}

// GetMintDecimals returns the decimals for the given SPL token mint.
// It uses an in-memory TTL cache to avoid hammering RPC.
func (c *RPCClient) GetMintDecimals(ctx context.Context, mint string) (uint8, error) {
	if mint == "" {
		return 0, errors.New("empty mint")
	}

	now := time.Now()
	c.mu.Lock()
	if cd, ok := c.decCache[mint]; ok && cd.expires.After(now) {
		c.mu.Unlock()
		return cd.value, nil
	}
	c.mu.Unlock()

	dec, err := c.fetchMintDecimals(ctx, mint)
	if err != nil {
		return 0, err
	}
	c.mu.Lock()
	c.decCache[mint] = cachedDecimals{
		value:   dec,
		expires: now.Add(c.ttl),
	}
	c.mu.Unlock()
	return dec, nil
}

// fetchMintDecimals hits getAccountInfo and decodes the SPL mint layout.
func (c *RPCClient) fetchMintDecimals(ctx context.Context, mint string) (uint8, error) {
	reqBody := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "getAccountInfo",
		"params": []any{
			mint,
			map[string]any{
				"encoding": "base64",
			},
		},
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(reqBody); err != nil {
		return 0, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, &buf)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return 0, fmt.Errorf("solana rpc http %d", resp.StatusCode)
	}

	var rpcResp getAccountInfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return 0, err
	}
	if rpcResp.Error != nil {
		return 0, fmt.Errorf("solana rpc error: %v", rpcResp.Error.Message)
	}
	if rpcResp.Result == nil || rpcResp.Result.Value == nil {
		return 0, errors.New("mint account not found")
	}

	data := rpcResp.Result.Value.Data
	if len(data) < 1 {
		return 0, errors.New("unexpected data format")
	}
	// Data is `[base64Data, "base64"]`
	str, ok := data[0].(string)
	if !ok {
		return 0, errors.New("unexpected data[0] type")
	}
	raw, err := base64.StdEncoding.DecodeString(str)
	if err != nil {
		return 0, err
	}
	// Minimal SPL Mint layout:
	//   mintAuthority:   COption<Pubkey> (4 + 32)
	//   supply:          u64            (8)
	//   decimals:        u8             (1)
	//   isInitialized:   bool           (1)
	//   freezeAuthority: COption<Pubkey> (4 + 32)
	// Total: 82 bytes, decimals at offset 44.
	if len(raw) < 45 {
		return 0, errors.New("mint data too short")
	}
	return uint8(raw[44]), nil
}

// MintAccountInfo is the parsed SPL mint account (authorities and decimals).
type MintAccountInfo struct {
	HasMintAuthority   bool
	HasFreezeAuthority bool
	Decimals          uint8
}

// GetMintAccountInfo returns mint authority, freeze authority, and decimals.
func (c *RPCClient) GetMintAccountInfo(ctx context.Context, mint string) (MintAccountInfo, error) {
	var out MintAccountInfo
	reqBody := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "getAccountInfo",
		"params":  []any{mint, map[string]any{"encoding": "base64"}},
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(reqBody); err != nil {
		return out, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, &buf)
	if err != nil {
		return out, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return out, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return out, fmt.Errorf("solana rpc http %d", resp.StatusCode)
	}
	var rpcResp getAccountInfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return out, err
	}
	if rpcResp.Error != nil {
		return out, fmt.Errorf("solana rpc error: %v", rpcResp.Error.Message)
	}
	if rpcResp.Result == nil || rpcResp.Result.Value == nil {
		return out, errors.New("mint account not found")
	}
	data := rpcResp.Result.Value.Data
	if len(data) < 1 {
		return out, errors.New("unexpected data format")
	}
	str, ok := data[0].(string)
	if !ok {
		return out, errors.New("unexpected data[0] type")
	}
	raw, err := base64.StdEncoding.DecodeString(str)
	if err != nil {
		return out, err
	}
	// SPL Mint: mint_authority 0..36, supply 36..44, decimals 44, is_initialized 45, freeze_authority 46..82
	if len(raw) < 50 {
		return out, errors.New("mint data too short")
	}
	out.HasMintAuthority = raw[0] != 0
	out.HasFreezeAuthority = raw[46] != 0
	out.Decimals = raw[44]
	return out, nil
}

type getAccountInfoResponse struct {
	Result *struct {
		Value *struct {
			Data []any `json:"data"`
		} `json:"value"`
	} `json:"result"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// GetTokenAccountBalance returns the raw amount (smallest units) of the token account.
// If the account does not exist or has no data, returns 0, nil or error.
func (c *RPCClient) GetTokenAccountBalance(ctx context.Context, tokenAccountBase58 string) (amount uint64, err error) {
	reqBody := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "getTokenAccountBalance",
		"params":  []any{tokenAccountBase58},
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(reqBody); err != nil {
		return 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, &buf)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return 0, fmt.Errorf("solana rpc http %d", resp.StatusCode)
	}
	var rpcResp struct {
		Result *struct {
			Value *struct {
				Amount   string `json:"amount"`
				Decimals uint8  `json:"decimals"`
			} `json:"value"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return 0, err
	}
	if rpcResp.Error != nil {
		return 0, fmt.Errorf("getTokenAccountBalance: %s", rpcResp.Error.Message)
	}
	if rpcResp.Result == nil || rpcResp.Result.Value == nil {
		return 0, nil // no account or no balance
	}
	amt, err := strconv.ParseUint(rpcResp.Result.Value.Amount, 10, 64)
	if err != nil {
		return 0, err
	}
	return amt, nil
}
