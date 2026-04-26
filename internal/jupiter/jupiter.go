package jupiter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"
)

type Client struct {
	http   *http.Client
	base   string
	apiKey string
}

func New() *Client {
	return &Client{
		http:   &http.Client{Timeout: 10 * time.Second},
		base:   "https://api.jup.ag",
		apiKey: os.Getenv("JUPITER_API_KEY"),
	}
}

// QuoteResponse holds the full quote from GET /quote. Raw is the complete JSON (required for POST /swap);
// OutAmount, InAmount, PriceImpact are parsed for convenience. Jupiter returns 422 if quoteResponse is incomplete.
type QuoteResponse struct {
	Raw         json.RawMessage `json:"-"` // full response; send as quoteResponse in Swap
	OutAmount   string         `json:"outAmount"`
	InAmount    string         `json:"inAmount"`
	PriceImpact string         `json:"priceImpactPct"` // e.g. "0.01"
}

// ParseAmountUint parses a Jupiter integer string amount (e.g. inAmount or
// outAmount) into a uint64. It returns 0 and an error on failure.
func ParseAmountUint(s string) (uint64, error) {
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, err
	}
	return v, nil
}

type QuoteParams struct {
	InputMint      string
	OutputMint     string
	Amount         uint64 // smallest units (lamports for SOL, token base units otherwise)
	SlippageBps    int
	OnlyDirect     bool
	MaxAccounts    int
	SwapMode       string // "ExactIn"
	PlatformFeeBps int
}

func (c *Client) Quote(ctx context.Context, p QuoteParams) (*QuoteResponse, error) {
	u, _ := url.Parse(c.base + "/swap/v1/quote")
	q := u.Query()
	q.Set("inputMint", p.InputMint)
	q.Set("outputMint", p.OutputMint)
	q.Set("amount", fmt.Sprintf("%d", p.Amount))
	if p.SlippageBps > 0 {
		q.Set("slippageBps", fmt.Sprintf("%d", p.SlippageBps))
	}
	if p.OnlyDirect {
		q.Set("onlyDirectRoutes", "true")
	}
	if p.MaxAccounts > 0 {
		q.Set("maxAccounts", fmt.Sprintf("%d", p.MaxAccounts))
	}
	if p.SwapMode != "" {
		q.Set("swapMode", p.SwapMode)
	}
	u.RawQuery = q.Encode()

	req, _ := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if c.apiKey != "" {
		req.Header.Set("x-api-key", c.apiKey)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("jupiter quote http %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	qr := &QuoteResponse{Raw: body}
	var aux struct {
		OutAmount   string `json:"outAmount"`
		InAmount    string `json:"inAmount"`
		PriceImpact string `json:"priceImpactPct"`
	}
	_ = json.Unmarshal(body, &aux)
	qr.OutAmount = aux.OutAmount
	qr.InAmount = aux.InAmount
	qr.PriceImpact = aux.PriceImpact
	return qr, nil
}

// swapRequestBody is the body for POST /swap/v1/swap. QuoteResponse must be the full JSON from GET /quote.
type swapRequestBody struct {
	QuoteResponse    json.RawMessage `json:"quoteResponse"`
	UserPublicKey    string          `json:"userPublicKey"`
	WrapAndUnwrapSol *bool           `json:"wrapAndUnwrapSol,omitempty"`
	DynamicCompute   *bool           `json:"dynamicComputeUnitLimit,omitempty"`
	PriorityLevel    string          `json:"priorityLevelWithMaxLamports,omitempty"`
}

// SwapRequest is optional overrides when calling Swap (e.g. WrapAndUnwrapSol).
type SwapRequest struct {
	WrapAndUnwrapSol *bool
	DynamicCompute   *bool
	PriorityLevel    string
}

// SwapResponse is the response from POST /swap/v1/swap.
type SwapResponse struct {
	SwapTransaction           string `json:"swapTransaction"`
	LastValidBlockHeight      uint64 `json:"lastValidBlockHeight"`
	PrioritizationFeeLamports uint64 `json:"prioritizationFeeLamports"`
}

// Swap builds a serialized swap transaction from a quote. Quote must contain the full Raw response from Quote();
// otherwise Jupiter returns 422 (incomplete quote).
func (c *Client) Swap(ctx context.Context, quote *QuoteResponse, userPublicKey string, opts *SwapRequest) (*SwapResponse, error) {
	if quote == nil || len(quote.Raw) == 0 {
		return nil, fmt.Errorf("quote is nil or empty")
	}
	wrap := true
	body := swapRequestBody{
		QuoteResponse:    quote.Raw,
		UserPublicKey:    userPublicKey,
		WrapAndUnwrapSol: &wrap,
	}
	if opts != nil {
		if opts.WrapAndUnwrapSol != nil {
			body.WrapAndUnwrapSol = opts.WrapAndUnwrapSol
		}
		if opts.DynamicCompute != nil {
			body.DynamicCompute = opts.DynamicCompute
		}
		if opts.PriorityLevel != "" {
			body.PriorityLevel = opts.PriorityLevel
		}
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/swap/v1/swap", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("x-api-key", c.apiKey)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		if len(body) > 0 {
			return nil, fmt.Errorf("jupiter swap http %d: %s", resp.StatusCode, bytes.TrimSpace(body))
		}
		return nil, fmt.Errorf("jupiter swap http %d", resp.StatusCode)
	}
	var sr SwapResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, err
	}
	return &sr, nil
}
