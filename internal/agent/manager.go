package agent

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/ShowBaba/tradebot/internal/executor"
	"github.com/ShowBaba/tradebot/internal/jupiter"
	"github.com/ShowBaba/tradebot/internal/logbus"
	"github.com/ShowBaba/tradebot/internal/token"
)

type Manager struct {
	mu            sync.RWMutex
	agents        map[string]*Agent
	deletedAgents []persistedAgent // history of deleted agents, never removed
	trades        []Trade
	jup           *jupiter.Client
	log           *logbus.Bus
	backend       StoreBackend
	dirty         bool

	// Optional: for live mode
	wallet           executor.Signer
	rpc              executor.TxSender
	liveCfg          executor.LiveConfig
	validatePosition func(ctx context.Context, walletPubkey, targetMint string, expectedTargetQty uint64) (ok bool, err error)
}

// NewManager creates a manager that persists state via backend. If backend is nil, save/load are no-ops.
func NewManager(jup *jupiter.Client, bus *logbus.Bus, backend StoreBackend) *Manager {
	m := &Manager{
		agents:  make(map[string]*Agent),
		jup:     jup,
		log:     bus,
		backend: backend,
	}
	go m.autosaveLoop()
	return m
}

// SetLiveDeps sets wallet, rpc, and optional position validator for live mode.
// validatePosition is called when restoring a live position; if nil, live positions are not restored.
func (m *Manager) SetLiveDeps(wallet executor.Signer, rpc executor.TxSender, cfg executor.LiveConfig, validatePosition func(ctx context.Context, walletPubkey, targetMint string, expectedTargetQty uint64) (ok bool, err error)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.wallet = wallet
	m.rpc = rpc
	m.liveCfg = cfg
	m.validatePosition = validatePosition
}

// Load restores state from disk. Call after SetLiveDeps when using live mode so validation can run.
func (m *Manager) Load() {
	m.load()
}

// execForMode returns the executor for the given mode.
func (m *Manager) execForMode(mode Mode) executor.Executor {
	if mode == ModeLive && m.wallet != nil && m.rpc != nil {
		return executor.NewLive(m.jup, m.wallet, m.rpc, m.liveCfg)
	}
	return executor.NewPaper()
}

func (m *Manager) Create(cfg Config) (*Agent, error) {
	m.mu.Lock()
	if cfg.ID == "" {
		m.mu.Unlock()
		return nil, errors.New("missing id")
	}
	if _, ok := m.agents[cfg.ID]; ok {
		m.mu.Unlock()
		return nil, errors.New("agent exists")
	}
	exec := m.execForMode(cfg.Mode)
	a := New(cfg, m.jup, exec, m.log)
	aID := cfg.ID
	a.emit("info", "agent created", nil)
	// hook trade recording
	a.onTrade = func(t Trade) {
		m.recordTrade(t)
	}
	m.agents[cfg.ID] = a
	m.dirty = true
	m.mu.Unlock()

	if err := m.save(); err != nil {
		log.Printf("agent %s created but failed to save state: %v", aID, err)
	} else {
		fmt.Println("saved")
	}
	log.Printf("agent %s created", aID)
	return a, nil
}

func (m *Manager) Get(id string) (*Agent, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	a, ok := m.agents[id]
	return a, ok
}

func (m *Manager) List() []*Agent {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*Agent, 0, len(m.agents))
	for _, a := range m.agents {
		out = append(out, a)
	}
	return out
}

func (m *Manager) Start(id string) error {
	a, ok := m.Get(id)
	if !ok {
		return errors.New("not found")
	}
	a.Start()
	m.mu.Lock()
	m.dirty = true
	m.mu.Unlock()
	return m.save()
}

func (m *Manager) Stop(id string) error {
	a, ok := m.Get(id)
	if !ok {
		return errors.New("not found")
	}
	a.Stop()
	m.mu.Lock()
	m.dirty = true
	m.mu.Unlock()
	return m.save()
}

// Delete removes an agent from the active list but keeps it in deleted history. Only allowed when stopped.
func (m *Manager) Delete(id string) error {
	m.mu.Lock()
	a, ok := m.agents[id]
	if !ok {
		m.mu.Unlock()
		return errors.New("not found")
	}
	a.mu.RLock()
	status := a.status
	a.mu.RUnlock()
	if status != StatusStopped {
		m.mu.Unlock()
		return errors.New("agent must be stopped before delete")
	}
	m.deletedAgents = append(m.deletedAgents, a.toPersisted())
	delete(m.agents, id)
	m.dirty = true
	m.mu.Unlock()
	if err := m.save(); err != nil {
		return err
	}
	log.Printf("agent %s deleted", id)
	return nil
}

// ResetStats clears paper PnL and loss counters for an agent.
func (m *Manager) ResetStats(id string) error {
	a, ok := m.Get(id)
	if !ok {
		return errors.New("not found")
	}
	a.mu.Lock()
	a.consecutiveLosses = 0
	a.totalPnLBase = 0
	a.lastPnlLog = time.Time{}
	a.lastPnlPct = 0
	a.mu.Unlock()
	a.emit("info", "stats reset", nil)

	m.mu.Lock()
	m.dirty = true
	m.mu.Unlock()
	return m.save()
}

// Trades returns a snapshot of recent trades.
func (m *Manager) Trades() []Trade {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Trade, len(m.trades))
	copy(out, m.trades)
	return out
}

// recordTrade is called by agents on enter/exit.
func (m *Manager) recordTrade(t Trade) {
	m.mu.Lock()
	m.trades = append(m.trades, t)
	// keep a moderate bound to avoid unbounded growth in memory
	const maxTrades = 1000
	if len(m.trades) > maxTrades {
		m.trades = m.trades[len(m.trades)-maxTrades:]
	}
	m.dirty = true
	m.mu.Unlock()
	_ = m.save()
}

func (m *Manager) snapshotStore() *Store {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s := &Store{
		Agents:        make([]persistedAgent, 0, len(m.agents)),
		DeletedAgents: m.deletedAgents,
		Trades:        make([]Trade, len(m.trades)),
	}
	for _, a := range m.agents {
		s.Agents = append(s.Agents, a.toPersisted())
	}
	copy(s.Trades, m.trades)
	return s
}

func (m *Manager) save() error {
	if m.backend == nil {
		return nil
	}
	s := m.snapshotStore()
	if err := m.backend.Save(s); err != nil {
		log.Printf("failed to save state: %v", err)
		return err
	}
	m.mu.Lock()
	m.dirty = false
	m.mu.Unlock()
	return nil
}

func (m *Manager) autosaveLoop() {
	t := time.NewTicker(10 * time.Second)
	defer t.Stop()
	for range t.C {
		m.mu.RLock()
		need := m.dirty
		m.mu.RUnlock()
		if need {
			_ = m.save()
		}
	}
}

// load restores agents and trades from persisted state. Agents are recreated
// in STOPPED state, and any open positions are cleared with a warning log to
// avoid phantom positions.
func (m *Manager) load() {
	if m.backend == nil {
		return
	}
	s, err := m.backend.Load()
	if err != nil {
		log.Printf("failed to load state: %v", err)
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, pa := range s.Agents {
		cfg := pa.Config
		exec := m.execForMode(cfg.Mode)
		a := New(cfg, m.jup, exec, m.log)
		a.status = StatusStopped
		a.consecutiveLosses = pa.ConsecutiveLosses
		a.totalPnLBase = pa.TotalPnLBase
		a.lastAction = pa.LastAction
		a.onTrade = func(t Trade) {
			m.recordTrade(t)
		}
		if pa.Position != nil {
			if cfg.Mode == ModeLive && m.wallet != nil && m.validatePosition != nil {
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				ok, err := m.validatePosition(ctx, m.wallet.PublicKeyBase58(), cfg.TargetMint, pa.Position.TargetQtySmallest)
				cancel()
				if err != nil || !ok {
					a.status = StatusNeedsManualAttention
					a.emit("error", "position validation failed on restart; needs manual attention", map[string]any{
						"err": err, "entryTime": pa.Position.EntryTime, "targetQtySmallest": pa.Position.TargetQtySmallest,
					})
				} else {
					entryBase := token.SmallestToFloat(pa.Position.EntryBaseSmallest, cfg.BaseDecimals)
					targetQty := token.SmallestToFloat(pa.Position.TargetQtySmallest, cfg.TargetDecimals)
					entryPrice := 0.0
					if targetQty > 0 {
						entryPrice = entryBase / targetQty
					}
					a.pos = &Position{
						EntryTime:          pa.Position.EntryTime,
						EntryBaseSmallest:  pa.Position.EntryBaseSmallest,
						TargetQtySmallest:  pa.Position.TargetQtySmallest,
						EntryBase:          entryBase,
						TargetQty:          targetQty,
						EntryPriceBasePerT: entryPrice,
					}
					a.status = StatusInPos
				}
			} else {
				a.emit("warn", "position cleared on restart (paper only)", map[string]any{
					"entryTime": pa.Position.EntryTime,
				})
			}
		}
		m.agents[cfg.ID] = a
	}
	m.deletedAgents = s.DeletedAgents
	m.trades = s.Trades
	log.Printf("loaded %d agents, %d deleted (history), %d trades from state", len(s.Agents), len(s.DeletedAgents), len(s.Trades))

	// One-time startup note: paper positions are not restored
	m.log.Publish(logbus.Event{
		Time:   time.Now(),
		Level:  "info",
		Agent:  "system",
		Msg:    "Paper positions are cleared on restart.",
		Fields: map[string]any{"reason": "paper_mode"},
	})
}
