package agent

import (
	"context"
	"math"
	"strconv"
	"sync"
	"time"

	"github.com/ShowBaba/tradebot/internal/executor"
	"github.com/ShowBaba/tradebot/internal/jupiter"
	"github.com/ShowBaba/tradebot/internal/logbus"
	"github.com/ShowBaba/tradebot/internal/token"
)

type Mode string

const (
	ModePaper Mode = "paper"
	ModeLive  Mode = "live"
)

type Config struct {
	ID string

	BaseMint   string // e.g. SOL mint
	TargetMint string // e.g. WAR mint

	// Mint decimals for base/target tokens.
	BaseDecimals   uint8
	TargetDecimals uint8

	// Amount in base smallest units (lamports if base is SOL)
	TradeSizeBaseSmallest uint64

	PollInterval   time.Duration
	Cooldown       time.Duration
	MaxSlippageBps int

	// Maximum allowed price impact before skipping entry (fraction, e.g. 0.01 = 1%).
	MaxPriceImpactFraction float64

	// Liquidity safety gate: minimum expected output in target smallest units to allow entry.
	// 0 means no minimum.
	MinOutTargetSmallest uint64
	// QuoteStabilityWindow: number of consecutive quotes to require before entry; stability is
	// checked when >= 2. 0 or 1 disables stability check.
	QuoteStabilityWindow int
	// MaxJitterPct: max allowed relative change in implied mid price between consecutive quotes (e.g. 0.5 = 0.5%).
	// Only used when QuoteStabilityWindow >= 2.
	MaxJitterPct float64

	TakeProfitPct float64 // e.g. 5 = 5%
	StopLossPct   float64 // e.g. 2 = 2%

	MaxConsecutiveLosses int
	MaxTotalLossBase     float64 // in base tokens (SOL), paper accounting

	// Per-agent risk (live or paper)
	MaxOpenTimeMinutes int      // auto-exit after N minutes in position; 0 = off
	MaxDailyLossBase   float64  // block new entries or kill if daily loss >= this; 0 = off
	MaxTradesPerHour   int      // skip entry if trades in last hour >= this; 0 = off
	AllowedMints       []string // if non-empty, base and target must be in list
	DeniedMints        []string // if non-empty, base and target must not be in list

	Mode Mode
}

type Status string

const (
	StatusStopped            Status = "stopped"
	StatusWaiting            Status = "waiting"
	StatusInPos              Status = "in_position"
	StatusCooldown           Status = "cooldown"
	StatusKilled             Status = "killed"
	StatusNeedsManualAttention Status = "needs_manual_attention"
)

type Position struct {
	EntryTime time.Time

	// Stored in smallest units for correctness.
	EntryBaseSmallest  uint64
	TargetQtySmallest  uint64
	EntryBase          float64 // base spent in base units
	TargetQty          float64 // target received in target units
	EntryPriceBasePerT float64 // base per target

	// Live: tx signature for entry (for trade record on exit)
	EntryTxSig string
}

type Agent struct {
	cfg Config
	jup *jupiter.Client
	exec executor.Executor
	log *logbus.Bus

	onTrade func(Trade)

	mu     sync.RWMutex
	status Status
	pos    *Position

	// risk tracking (paper)
	consecutiveLosses int
	totalPnLBase      float64 // positive/negative in SOL
	lastAction        time.Time

	// hold log throttling
	lastPnlLog time.Time
	lastPnlPct float64

	// liquidity gate: last N implied mid prices (base per target) for stability check
	entryQuotePrices     []float64
	lastLiquidityGateLog time.Time

	// Live: last execution error (for snapshot)
	lastError string

	// Per-agent risk state
	dailyPnLBase    float64
	dailyPnLResetAt time.Time
	recentTrades    []time.Time // times of last trades for rate limit (cap 100)

	cancel context.CancelFunc
}

func New(cfg Config, jup *jupiter.Client, exec executor.Executor, bus *logbus.Bus) *Agent {
	return &Agent{
		cfg:    cfg,
		jup:    jup,
		exec:   exec,
		log:    bus,
		status: StatusStopped,
	}
}

func (a *Agent) ID() string { return a.cfg.ID }

func (a *Agent) Snapshot() map[string]any {
	a.mu.RLock()
	defer a.mu.RUnlock()
	tradeSizeBase := token.SmallestToFloat(a.cfg.TradeSizeBaseSmallest, a.cfg.BaseDecimals)
	out := map[string]any{
		"id":                         a.cfg.ID,
		"baseMint":                   a.cfg.BaseMint,
		"targetMint":                 a.cfg.TargetMint,
		"status":                     a.status,
		"mode":                       a.cfg.Mode,
		"consecutiveLosses":          a.consecutiveLosses,
		"totalPnLBase":               a.totalPnLBase,
		"tradeSizeBase":              tradeSizeBase,
		"takeProfitPct":              a.cfg.TakeProfitPct,
		"stopLossPct":                a.cfg.StopLossPct,
		"tradeSizeBaseSmallest":       a.cfg.TradeSizeBaseSmallest,
		"baseDecimals":                a.cfg.BaseDecimals,
		"targetDecimals":              a.cfg.TargetDecimals,
		"maxPriceImpactFraction":      a.cfg.MaxPriceImpactFraction,
		"maxPriceImpactPercent":       a.cfg.MaxPriceImpactFraction * 100,
		"tradeSizeBaseSmallestDisplay": tradeSizeBase,
		"lastError":                    a.lastError,
	}
	if a.pos != nil {
		out["position"] = map[string]any{
			"entryTime":          a.pos.EntryTime,
			"entryBase":          a.pos.EntryBase,
			"entryPriceBasePerT": a.pos.EntryPriceBasePerT,
			"targetQty":          a.pos.TargetQty,
			"entryTxSig":         a.pos.EntryTxSig,
		}
	}
	return out
}

func (a *Agent) Start() {
	a.mu.Lock()
	if a.status != StatusStopped {
		a.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel
	a.status = StatusWaiting
	a.mu.Unlock()

	a.emit("info", "started", nil)
	go a.loop(ctx)
}

func (a *Agent) Stop() {
	a.mu.Lock()
	if a.cancel != nil {
		a.cancel()
	}
	a.status = StatusStopped
	a.pos = nil
	a.mu.Unlock()
	a.emit("info", "stopped", nil)
}

func (a *Agent) Kill(reason string) {
	a.mu.Lock()
	if a.cancel != nil {
		a.cancel()
	}
	a.status = StatusKilled
	a.mu.Unlock()
	a.emit("error", "killed: "+reason, nil)
}

func (a *Agent) loop(ctx context.Context) {
	t := time.NewTicker(a.cfg.PollInterval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			a.tick(ctx)
		}
	}
}

// tick uses quotes to decide entry/exit in paper mode
func (a *Agent) tick(ctx context.Context) {
	a.mu.RLock()
	status := a.status
	pos := a.pos
	last := a.lastAction
	baseDec := a.cfg.BaseDecimals
	targetDec := a.cfg.TargetDecimals
	maxImpact := a.cfg.MaxPriceImpactFraction
	a.mu.RUnlock()

	if status == StatusKilled || status == StatusStopped {
		return
	}

	// cooldown gate
	if status == StatusCooldown && time.Since(last) < a.cfg.Cooldown {
		return
	}
	if status == StatusCooldown && time.Since(last) >= a.cfg.Cooldown {
		a.setStatus(StatusWaiting)
	}

	if pos == nil {
		// ENTRY: per-agent risk gates
		a.mu.Lock()
		now := time.Now()
		if a.dailyPnLResetAt.IsZero() || now.Sub(a.dailyPnLResetAt) >= 24*time.Hour {
			a.dailyPnLBase = 0
			a.dailyPnLResetAt = now
		}
		if a.cfg.MaxDailyLossBase > 0 && a.dailyPnLBase <= -math.Abs(a.cfg.MaxDailyLossBase) {
			a.mu.Unlock()
			a.emitLiquidityGateThrottled("skip entry: max daily loss reached", map[string]any{"dailyPnLBase": a.dailyPnLBase})
			return
		}
		// Trim recentTrades to last hour and count
		cutoff := now.Add(-1 * time.Hour)
		n := 0
		for _, t := range a.recentTrades {
			if t.After(cutoff) {
				n++
			}
		}
		if n > 0 {
			newRecent := make([]time.Time, 0, len(a.recentTrades))
			for _, t := range a.recentTrades {
				if t.After(cutoff) {
					newRecent = append(newRecent, t)
				}
			}
			a.recentTrades = newRecent
		}
		if a.cfg.MaxTradesPerHour > 0 && n >= a.cfg.MaxTradesPerHour {
			a.mu.Unlock()
			a.emitLiquidityGateThrottled("skip entry: max trades per hour", map[string]any{"count": n})
			return
		}
		if !a.checkMintAllowDeny(a.cfg.BaseMint) || !a.checkMintAllowDeny(a.cfg.TargetMint) {
			a.mu.Unlock()
			a.emit("warn", "skip entry: mint not in allowlist or in denylist", nil)
			return
		}
		a.mu.Unlock()

		// - get quote for Base -> Target
		// - liquidity gate: impact, outAmount, minOut, quote stability (optional)
		q, err := a.jup.Quote(ctx, jupiter.QuoteParams{
			InputMint:   a.cfg.BaseMint,
			OutputMint:  a.cfg.TargetMint,
			Amount:      a.cfg.TradeSizeBaseSmallest,
			SlippageBps: a.cfg.MaxSlippageBps,
			SwapMode:    "ExactIn",
		})
		if err != nil {
			a.emit("warn", "quote failed (entry)", map[string]any{"err": err.Error()})
			return
		}

		impact := parseFloat(q.PriceImpact)
		if maxImpact <= 0 {
			maxImpact = 0.01 // default 1%
		}
		if impact > maxImpact {
			a.emitLiquidityGateThrottled("skip entry: high impact", map[string]any{"impactPct": impact})
			return
		}

		outAmtSmallest, err := jupiter.ParseAmountUint(q.OutAmount)
		if err != nil || outAmtSmallest == 0 {
			a.emit("warn", "bad quote outAmount", map[string]any{"outAmount": q.OutAmount})
			return
		}

		entryBase := token.SmallestToFloat(a.cfg.TradeSizeBaseSmallest, baseDec)
		targetQty := token.SmallestToFloat(outAmtSmallest, targetDec)
		if targetQty <= 0 {
			a.emit("warn", "computed zero targetQty", map[string]any{"outAmount": q.OutAmount})
			return
		}
		entryPrice := entryBase / targetQty

		// Liquidity gate: minOut (target smallest units)
		if a.cfg.MinOutTargetSmallest > 0 && outAmtSmallest < a.cfg.MinOutTargetSmallest {
			a.emitLiquidityGateThrottled("skip entry: out below minimum", map[string]any{
				"outAmount": outAmtSmallest, "minOutTargetSmallest": a.cfg.MinOutTargetSmallest,
			})
			return
		}

		// Quote stability: require implied mid price stable across consecutive quotes
		if a.cfg.QuoteStabilityWindow >= 2 && a.cfg.MaxJitterPct > 0 {
			a.mu.Lock()
			a.entryQuotePrices = append(a.entryQuotePrices, entryPrice)
			window := a.cfg.QuoteStabilityWindow
			if window < 2 {
				window = 2
			}
			if len(a.entryQuotePrices) > window {
				a.entryQuotePrices = a.entryQuotePrices[len(a.entryQuotePrices)-window:]
			}
			prices := make([]float64, len(a.entryQuotePrices))
			copy(prices, a.entryQuotePrices)
			a.mu.Unlock()

			if len(prices) >= 2 {
				prev, cur := prices[len(prices)-2], prices[len(prices)-1]
				if prev > 0 {
					jitterPct := math.Abs(cur-prev) / prev * 100
					if jitterPct > a.cfg.MaxJitterPct {
						a.emitLiquidityGateThrottled("skip entry: quote jitter too high", map[string]any{
							"jitterPct": jitterPct, "maxJitterPct": a.cfg.MaxJitterPct,
						})
						return
					}
				}
			} else {
				// not enough quotes yet, skip this tick (will retry next poll)
				return
			}
		}

		execCfg := executor.Config{
			BaseMint:              a.cfg.BaseMint,
			TargetMint:             a.cfg.TargetMint,
			BaseDecimals:           baseDec,
			TargetDecimals:         targetDec,
			TradeSizeBaseSmallest:  a.cfg.TradeSizeBaseSmallest,
			MaxSlippageBps:         a.cfg.MaxSlippageBps,
		}
		res := a.exec.Enter(ctx, execCfg, q)
		if res.Error != nil {
			a.mu.Lock()
			a.lastError = res.Error.Error()
			a.mu.Unlock()
			a.emit("warn", "enter failed", map[string]any{"err": res.Error.Error()})
			return
		}
		a.mu.Lock()
		a.lastError = ""
		a.mu.Unlock()

		outFromExec := res.ExpectedOut
		if res.ActualOut > 0 {
			outFromExec = res.ActualOut
		}
		entryBase = token.SmallestToFloat(a.cfg.TradeSizeBaseSmallest, baseDec)
		targetQty = token.SmallestToFloat(outFromExec, targetDec)
		if targetQty <= 0 {
			return
		}
		entryPrice = entryBase / targetQty

		a.mu.Lock()
		a.pos = &Position{
			EntryTime:          time.Now(),
			EntryBaseSmallest:  a.cfg.TradeSizeBaseSmallest,
			TargetQtySmallest:  outFromExec,
			EntryBase:          entryBase,
			TargetQty:          targetQty,
			EntryPriceBasePerT: entryPrice,
			EntryTxSig:         res.TxSignature,
		}
		a.status = StatusInPos
		a.lastAction = time.Now()
		a.entryQuotePrices = nil
		a.mu.Unlock()

		a.emit("info", "entered", map[string]any{
			"spentBase":            entryBase,
			"gotTargetQty":         targetQty,
			"entryPriceBasePerTgt": entryPrice,
			"impactPct":            impact,
			"txSig":                res.TxSignature,
		})

		if a.onTrade != nil {
			a.mu.Lock()
			a.recentTrades = append(a.recentTrades, time.Now())
			if len(a.recentTrades) > 100 {
				a.recentTrades = a.recentTrades[len(a.recentTrades)-100:]
			}
			a.mu.Unlock()
			trade := Trade{
				Time:              time.Now(),
				AgentID:           a.cfg.ID,
				Action:            "enter",
				BaseMint:          a.cfg.BaseMint,
				TargetMint:        a.cfg.TargetMint,
				EntryBaseSmallest: a.cfg.TradeSizeBaseSmallest,
				TargetQtySmallest: outFromExec,
				Reason:            "enter_position",
				EntryTxSig:        res.TxSignature,
			}
			go a.onTrade(trade)
		}
		return
	}

	// EXIT valuation: quote Target -> Base using exact-in of held target amount.
	q2, err := a.jup.Quote(ctx, jupiter.QuoteParams{
		InputMint:   a.cfg.TargetMint,
		OutputMint:  a.cfg.BaseMint,
		Amount:      pos.TargetQtySmallest,
		SlippageBps: a.cfg.MaxSlippageBps,
		SwapMode:    "ExactIn",
	})
	if err != nil {
		a.emit("warn", "quote failed (exit valuation)", map[string]any{"err": err.Error()})
		return
	}

	outNowSmallest, err := jupiter.ParseAmountUint(q2.OutAmount)
	if err != nil || outNowSmallest == 0 {
		a.emit("warn", "bad quote outAmount (exit valuation)", map[string]any{"outAmount": q2.OutAmount})
		return
	}

	valueNowBase := token.SmallestToFloat(outNowSmallest, baseDec)
	entryBase := token.SmallestToFloat(pos.EntryBaseSmallest, baseDec)
	if entryBase <= 0 {
		return
	}
	pnlPct := (valueNowBase - entryBase) / entryBase * 100.0

	// exit rules: if TP or SL or max open time, execute exit then close position
	if pnlPct >= a.cfg.TakeProfitPct {
		a.executeExitAndClose(ctx, pos, q2, valueNowBase, pnlPct, "take_profit")
		return
	}
	if a.cfg.StopLossPct > 0 && pnlPct <= -math.Abs(a.cfg.StopLossPct) {
		a.executeExitAndClose(ctx, pos, q2, valueNowBase, pnlPct, "stop_loss")
		return
	}
	if a.cfg.MaxOpenTimeMinutes > 0 && time.Since(pos.EntryTime) >= time.Duration(a.cfg.MaxOpenTimeMinutes)*time.Minute {
		a.executeExitAndClose(ctx, pos, q2, valueNowBase, pnlPct, "max_open_time")
		return
	}

	// Throttled "hold" logging.
	now := time.Now()
	a.mu.Lock()
	shouldLog := false
	if a.lastPnlLog.IsZero() || now.Sub(a.lastPnlLog) >= 15*time.Second || math.Abs(pnlPct-a.lastPnlPct) >= 0.05 {
		shouldLog = true
		a.lastPnlLog = now
		a.lastPnlPct = pnlPct
	}
	a.mu.Unlock()
	if shouldLog {
		a.emit("info", "hold", map[string]any{
			"valueNowBase": valueNowBase,
			"pnlPct":       pnlPct,
		})
	}
}

// executeExitAndClose calls the executor to perform exit, then closes the position and records the trade.
func (a *Agent) executeExitAndClose(ctx context.Context, pos *Position, exitQuote *jupiter.QuoteResponse, quoteValueNowBase, pnlPct float64, reason string) {
	execCfg := executor.Config{
		BaseMint:              a.cfg.BaseMint,
		TargetMint:             a.cfg.TargetMint,
		BaseDecimals:           a.cfg.BaseDecimals,
		TargetDecimals:         a.cfg.TargetDecimals,
		TradeSizeBaseSmallest:  pos.EntryBaseSmallest,
		MaxSlippageBps:         a.cfg.MaxSlippageBps,
	}
	execPos := &executor.Position{
		EntryTime:         pos.EntryTime,
		EntryBaseSmallest: pos.EntryBaseSmallest,
		TargetQtySmallest: pos.TargetQtySmallest,
	}
	res := a.exec.Exit(ctx, execCfg, exitQuote, execPos)
	if res.Error != nil {
		a.mu.Lock()
		a.lastError = res.Error.Error()
		a.mu.Unlock()
		a.emit("warn", "exit failed", map[string]any{"err": res.Error.Error()})
		return
	}
	a.mu.Lock()
	a.lastError = ""
	a.mu.Unlock()

	valueNowBase := quoteValueNowBase
	if res.ActualOut > 0 {
		valueNowBase = token.SmallestToFloat(res.ActualOut, a.cfg.BaseDecimals)
	} else if res.ExpectedOut > 0 {
		valueNowBase = token.SmallestToFloat(res.ExpectedOut, a.cfg.BaseDecimals)
	}
	entryBase := token.SmallestToFloat(pos.EntryBaseSmallest, a.cfg.BaseDecimals)
	if entryBase > 0 {
		pnlPct = (valueNowBase - entryBase) / entryBase * 100.0
	}
	a.closePosition(valueNowBase, pnlPct, reason, &res)
}

func (a *Agent) closePosition(valueNowSOL, pnlPct float64, reason string, exitResult *executor.ExecutionResult) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.pos == nil {
		return
	}

	pnlSOL := valueNowSOL - a.pos.EntryBase
	a.totalPnLBase += pnlSOL
	a.dailyPnLBase += pnlSOL
	if pnlSOL < 0 {
		a.consecutiveLosses++
	} else {
		a.consecutiveLosses = 0
	}
	// Rate limit tracking
	a.recentTrades = append(a.recentTrades, time.Now())
	if len(a.recentTrades) > 100 {
		a.recentTrades = a.recentTrades[len(a.recentTrades)-100:]
	}

	feesLamports := uint64(0)
	exitTxSig := ""
	if exitResult != nil {
		feesLamports = exitResult.FeesLamports
		exitTxSig = exitResult.TxSignature
	}

	a.emit("info", "exit: "+reason, map[string]any{
		"exitValueSOL":      valueNowSOL,
		"pnlSOL":            pnlSOL,
		"pnlPct":            pnlPct,
		"totalPnLSOL":       a.totalPnLBase,
		"consecutiveLosses": a.consecutiveLosses,
		"txSig":             exitTxSig,
	})

	entryBaseSmallest := a.pos.EntryBaseSmallest
	targetQtySmallest := a.pos.TargetQtySmallest
	entryTxSig := a.pos.EntryTxSig
	a.pos = nil
	a.status = StatusCooldown
	a.lastAction = time.Now()

	// risk kill checks
	if a.cfg.MaxConsecutiveLosses > 0 && a.consecutiveLosses >= a.cfg.MaxConsecutiveLosses {
		go a.Kill("max consecutive losses reached")
		return
	}
	if a.cfg.MaxTotalLossBase > 0 && a.totalPnLBase <= -math.Abs(a.cfg.MaxTotalLossBase) {
		go a.Kill("max total loss reached")
		return
	}

	if a.onTrade != nil {
		exitSmallest := token.FloatToSmallest(valueNowSOL, a.cfg.BaseDecimals)
		pnlBaseSmallest := int64(exitSmallest) - int64(entryBaseSmallest)
		trade := Trade{
			Time:              time.Now(),
			AgentID:           a.cfg.ID,
			Action:            "exit",
			BaseMint:          a.cfg.BaseMint,
			TargetMint:        a.cfg.TargetMint,
			EntryBaseSmallest: entryBaseSmallest,
			TargetQtySmallest: targetQtySmallest,
			ExitBaseSmallest:  exitSmallest,
			PnlBaseSmallest:   pnlBaseSmallest,
			PnlPct:            pnlPct,
			Reason:            reason,
			EntryTxSig:        entryTxSig,
			ExitTxSig:         exitTxSig,
			FeesLamports:      feesLamports,
		}
		go a.onTrade(trade)
	}
}

func (a *Agent) setStatus(s Status) {
	a.mu.Lock()
	a.status = s
	a.mu.Unlock()
}

func (a *Agent) emit(level, msg string, fields any) {
	a.log.Publish(logbus.Event{
		Time:   time.Now(),
		Level:  level,
		Agent:  a.cfg.ID,
		Msg:    msg,
		Fields: fields,
	})
}

func parseFloat(s string) float64 {
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

// emitLiquidityGateThrottled logs when entry is skipped due to liquidity gate; at most once per 30s per agent.
func (a *Agent) emitLiquidityGateThrottled(msg string, fields any) {
	a.mu.Lock()
	now := time.Now()
	shouldLog := a.lastLiquidityGateLog.IsZero() || now.Sub(a.lastLiquidityGateLog) >= 30*time.Second
	if shouldLog {
		a.lastLiquidityGateLog = now
	}
	a.mu.Unlock()
	if shouldLog {
		a.emit("warn", "liquidity gate: "+msg, fields)
	}
}

// checkMintAllowDeny returns false if mint is disallowed by allowlist/denylist.
func (a *Agent) checkMintAllowDeny(mint string) bool {
	if len(a.cfg.AllowedMints) > 0 {
		for _, m := range a.cfg.AllowedMints {
			if m == mint {
				return true
			}
		}
		return false
	}
	if len(a.cfg.DeniedMints) > 0 {
		for _, m := range a.cfg.DeniedMints {
			if m == mint {
				return false
			}
		}
	}
	return true
}

// SOL-only MVP helper: lamports to SOL
func lamportsToSOL(lamports uint64) float64 {
	return float64(lamports) / 1_000_000_000.0
}
