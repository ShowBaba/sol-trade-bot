package ui

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/ShowBaba/tradebot/internal/agent"
	"github.com/ShowBaba/tradebot/internal/logbus"
	"github.com/ShowBaba/tradebot/internal/token"
)

// WalletProvider is optional; when set, GET /api/wallet returns pubkey and SOL balance.
type WalletProvider interface {
	PublicKey() string
	Balance(ctx context.Context) (lamports uint64, err error)
}

// LogHistoryProvider returns persisted log history (e.g. from Redis). When set, GET /api/logs/history uses it instead of in-memory bus.
type LogHistoryProvider interface {
	GetHistory(limit int) ([]logbus.Event, error)
}

type Server struct {
	mgr       *agent.Manager
	bus       *logbus.Bus
	tokens    *token.Service
	wallet    WalletProvider
	logStore  LogHistoryProvider
	staticDir string // if set, serve SPA from this dir (for production/Docker)
}

func New(mgr *agent.Manager, bus *logbus.Bus, tokens *token.Service, wallet WalletProvider) *Server {
	return &Server{mgr: mgr, bus: bus, tokens: tokens, wallet: wallet}
}

// SetLogStore sets the optional log history provider (e.g. Redis). When set, /api/logs/history returns from it (last 2000 entries).
func (s *Server) SetLogStore(store LogHistoryProvider) {
	s.logStore = store
}

// SetStaticDir sets the directory for serving the React SPA (e.g. web/dist). When set, non-API requests are served from this dir; missing paths fall back to index.html.
func (s *Server) SetStaticDir(dir string) {
	s.staticDir = strings.TrimSuffix(dir, "/")
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	// agents API
	mux.HandleFunc("/api/agents", s.handleAgents)
	mux.HandleFunc("/api/agents/create", s.handleCreateAgent)
	mux.HandleFunc("/api/agents/start", s.handleStartAgent)
	mux.HandleFunc("/api/agents/stop", s.handleStopAgent)

	// lifecycle
	mux.HandleFunc("/api/agents/delete", s.handleDeleteAgent)
	mux.HandleFunc("/api/agents/reset", s.handleResetAgent)

	// trades
	mux.HandleFunc("/api/trades", s.handleTrades)
	mux.HandleFunc("/api/trades.csv", s.handleTradesCSV)

	// wallet (read-only)
	mux.HandleFunc("/api/wallet", s.handleWallet)

	// logs stream
	mux.HandleFunc("/api/logs", s.handleLogsSSE)
	mux.HandleFunc("/api/logs/history", s.handleLogsHistory)
	mux.HandleFunc("/api/logs/clear", s.handleLogsClear)

	if s.staticDir != "" {
		fs := http.FileServer(http.Dir(s.staticDir))
		mux.Handle("/", spaHandler{root: s.staticDir, fileServer: fs})
	}

	return mux
}

// spaHandler serves static files and falls back to index.html for SPA routing.
type spaHandler struct {
	root       string
	fileServer http.Handler
}

func (h spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "" || r.URL.Path == "/" {
		r.URL.Path = "/"
		http.ServeFile(w, r, path.Join(h.root, "index.html"))
		return
	}
	p := path.Join(h.root, path.Clean("/"+r.URL.Path))
	if f, err := os.Stat(p); err == nil && !f.IsDir() {
		http.ServeFile(w, r, p)
		return
	}
	http.ServeFile(w, r, path.Join(h.root, "index.html"))
}

func (s *Server) handleAgents(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "method not allowed", 405)
		return
	}
	list := s.mgr.List()
	out := make([]map[string]any, 0, len(list))
	for _, a := range list {
		out = append(out, a.Snapshot())
	}
	writeJSON(w, out)
}

type createReq struct {
	ID string `json:"id"`

	BaseMint   string `json:"baseMint"`
	TargetMint string `json:"targetMint"`

	TradeSizeLamports uint64  `json:"tradeSizeLamports"`
	PollSeconds       int     `json:"pollSeconds"`
	CooldownSeconds   int     `json:"cooldownSeconds"`
	MaxSlippageBps    int     `json:"maxSlippageBps"`
	TakeProfitPct     float64 `json:"takeProfitPct"`
	StopLossPct       float64 `json:"stopLossPct"`

	MaxConsecutiveLosses int     `json:"maxConsecutiveLosses"`
	MaxTotalLossSOL      float64 `json:"maxTotalLossSOL"`

	MaxPriceImpactPct    float64  `json:"maxPriceImpactPct"`
	MinOutTargetSmallest uint64   `json:"minOutTargetSmallest"`
	QuoteStabilityWindow int      `json:"quoteStabilityWindow"`
	MaxJitterPct         float64  `json:"maxJitterPct"`

	Mode string `json:"mode"` // "paper" or "live"

	MaxOpenTimeMinutes int      `json:"maxOpenTimeMinutes"`
	MaxDailyLossSOL    float64  `json:"maxDailyLossSOL"`
	MaxTradesPerHour   int      `json:"maxTradesPerHour"`
	AllowedMints      []string `json:"allowedMints"`
	DeniedMints        []string `json:"deniedMints"`
}

func (s *Server) handleCreateAgent(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	var req createReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	if err := validateCreateReq(req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	baseDecimals, err := s.tokens.Decimals(r.Context(), req.BaseMint)
	if err != nil {
		http.Error(w, "failed to fetch base mint decimals: "+err.Error(), 502)
		return
	}
	targetDecimals, err := s.tokens.Decimals(r.Context(), req.TargetMint)
	if err != nil {
		http.Error(w, "failed to fetch target mint decimals: "+err.Error(), 502)
		return
	}

	maxImpactFraction := req.MaxPriceImpactPct / 100.0

	id := req.ID
	if id == "" {
		id = defaultAgentID(req.BaseMint, req.TargetMint)
	}

	cfg := agent.Config{
		ID: id,

		BaseMint:   req.BaseMint,
		TargetMint: req.TargetMint,

		BaseDecimals:   baseDecimals,
		TargetDecimals: targetDecimals,

		TradeSizeBaseSmallest: req.TradeSizeLamports,
		PollInterval:          time.Duration(req.PollSeconds) * time.Second,
		Cooldown:              time.Duration(req.CooldownSeconds) * time.Second,
		MaxSlippageBps:        req.MaxSlippageBps,

		MaxPriceImpactFraction: maxImpactFraction,
		MinOutTargetSmallest:   req.MinOutTargetSmallest,
		QuoteStabilityWindow:   req.QuoteStabilityWindow,
		MaxJitterPct:           req.MaxJitterPct,

		TakeProfitPct: req.TakeProfitPct,
		StopLossPct:   req.StopLossPct,

		MaxConsecutiveLosses: req.MaxConsecutiveLosses,
		MaxTotalLossBase:     req.MaxTotalLossSOL,

		MaxOpenTimeMinutes: req.MaxOpenTimeMinutes,
		MaxDailyLossBase:   req.MaxDailyLossSOL,
		MaxTradesPerHour:   req.MaxTradesPerHour,
		AllowedMints:       req.AllowedMints,
		DeniedMints:        req.DeniedMints,

		Mode: agent.ModePaper,
	}
	if req.Mode == "live" {
		cfg.Mode = agent.ModeLive
	}

	a, err := s.mgr.Create(cfg)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	writeJSON(w, a.Snapshot())
}

func (s *Server) handleDeleteAgent(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "missing id", 400)
		return
	}
	if err := s.mgr.Delete(id); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) handleResetAgent(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "missing id", 400)
		return
	}
	if err := s.mgr.ResetStats(id); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) handleTrades(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "method not allowed", 405)
		return
	}
	writeJSON(w, s.mgr.Trades())
}

func (s *Server) handleWallet(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "method not allowed", 405)
		return
	}
	out := map[string]any{"pubkey": "", "solBalance": 0.0, "configured": false}
	if s.wallet != nil {
		out["pubkey"] = s.wallet.PublicKey()
		out["configured"] = true
		lamports, err := s.wallet.Balance(r.Context())
		if err != nil {
			log.Printf("wallet balance fetch failed: %v", err)
		}
		out["solBalance"] = float64(lamports) / 1e9
	}
	writeJSON(w, out)
}

func (s *Server) handleTradesCSV(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "method not allowed", 405)
		return
	}
	trades := s.mgr.Trades()
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=trades.csv")
	wr := csv.NewWriter(w)
	_ = wr.Write([]string{"time", "agentID", "action", "baseMint", "targetMint", "entryBaseSmallest", "targetQtySmallest", "exitBaseSmallest", "pnlBaseSmallest", "pnlPct", "reason", "entryTxSig", "exitTxSig", "feesLamports"})
	for _, t := range trades {
		_ = wr.Write([]string{
			t.Time.Format(time.RFC3339),
			t.AgentID,
			t.Action,
			t.BaseMint,
			t.TargetMint,
			strconv.FormatUint(t.EntryBaseSmallest, 10),
			strconv.FormatUint(t.TargetQtySmallest, 10),
			strconv.FormatUint(t.ExitBaseSmallest, 10),
			strconv.FormatInt(t.PnlBaseSmallest, 10),
			strconv.FormatFloat(t.PnlPct, 'f', 4, 64),
			t.Reason,
			t.EntryTxSig,
			t.ExitTxSig,
			strconv.FormatUint(t.FeesLamports, 10),
		})
	}
	wr.Flush()
}

func shortMint(m string) string {
	if len(m) <= 4 {
		return m
	}
	return m[:4]
}

func defaultAgentID(baseMint, targetMint string) string {
	ts := time.Now().Unix()
	return fmt.Sprintf("%s-%s-%d", shortMint(baseMint), shortMint(targetMint), ts)
}

// validateCreateReq returns an error if the create request is invalid.
func validateCreateReq(req createReq) error {
	if req.BaseMint == "" {
		return fmt.Errorf("missing baseMint")
	}
	if req.TargetMint == "" {
		return fmt.Errorf("missing targetMint")
	}
	if req.TakeProfitPct < 0 || req.TakeProfitPct > 100 {
		return fmt.Errorf("takeProfitPct must be between 0 and 100 (percent, e.g. 5 = 5%%)")
	}
	if req.StopLossPct < 0 || req.StopLossPct > 100 {
		return fmt.Errorf("stopLossPct must be between 0 and 100 (percent, e.g. 2 = 2%%)")
	}
	if req.PollSeconds < 1 || req.PollSeconds > 86400 {
		return fmt.Errorf("pollSeconds must be between 1 and 86400")
	}
	if req.CooldownSeconds < 0 || req.CooldownSeconds > 86400 {
		return fmt.Errorf("cooldownSeconds must be between 0 and 86400")
	}
	if req.MaxSlippageBps < 0 || req.MaxSlippageBps > 10000 {
		return fmt.Errorf("maxSlippageBps must be between 0 and 10000")
	}
	if req.Mode != "" && req.Mode != "paper" && req.Mode != "live" {
		return fmt.Errorf("mode must be paper or live")
	}
	return nil
}

func (s *Server) handleStartAgent(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "missing id", 400)
		return
	}
	if err := s.mgr.Start(id); err != nil {
		http.Error(w, err.Error(), 404)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) handleStopAgent(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "missing id", 400)
		return
	}
	if err := s.mgr.Stop(id); err != nil {
		http.Error(w, err.Error(), 404)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) handleLogsHistory(w http.ResponseWriter, r *http.Request) {
	if s.logStore != nil {
		events, err := s.logStore.GetHistory(2000)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		writeJSON(w, events)
		return
	}
	writeJSON(w, s.bus.History())
}

func (s *Server) handleLogsClear(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	// Clear only in-memory buffer; persisted logs (e.g. Redis) are never cleared.
	s.bus.Clear()
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) handleLogsSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", 500)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch, cancel := s.bus.Subscribe(200)
	defer cancel()

	// keep-alive pings
	ping := time.NewTicker(15 * time.Second)
	defer ping.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case e := <-ch:
			b, _ := json.Marshal(e)
			fmt.Fprintf(w, "data: %s\n\n", b)
			flusher.Flush()
		case <-ping.C:
			fmt.Fprintf(w, ": ping\n\n")
			flusher.Flush()
		}
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
