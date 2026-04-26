package agent

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

// Trade represents a single trade event (enter or exit), paper or live.
type Trade struct {
	Time    time.Time `json:"time"`
	AgentID string    `json:"agentID"`
	Action  string    `json:"action"` // "enter" or "exit"

	BaseMint   string `json:"baseMint"`
	TargetMint string `json:"targetMint"`

	EntryBaseSmallest uint64 `json:"entryBaseSmallest"`
	TargetQtySmallest uint64 `json:"targetQtySmallest"`

	ExitBaseSmallest uint64 `json:"exitBaseSmallest"`
	PnlBaseSmallest  int64  `json:"pnlBaseSmallest"`
	PnlPct           float64 `json:"pnlPct"`
	Reason           string  `json:"reason"`

	// Live mode: tx signatures and fees
	EntryTxSig   string `json:"entryTxSig,omitempty"`
	ExitTxSig    string `json:"exitTxSig,omitempty"`
	FeesLamports uint64 `json:"feesLamports,omitempty"`
}

type persistedPosition struct {
	EntryTime         time.Time `json:"entryTime"`
	EntryBaseSmallest uint64    `json:"entryBaseSmallest"`
	TargetQtySmallest uint64    `json:"targetQtySmallest"`
}

type persistedAgent struct {
	Config            Config             `json:"config"`
	Status            Status             `json:"status"`
	Position          *persistedPosition `json:"position,omitempty"`
	ConsecutiveLosses int                `json:"consecutiveLosses"`
	TotalPnLBase      float64            `json:"totalPnLBase"`
	LastAction        time.Time          `json:"lastAction"`
}

// Store is the snapshot persisted to disk or Redis.
type Store struct {
	Agents        []persistedAgent `json:"agents"`
	DeletedAgents []persistedAgent `json:"deletedAgents,omitempty"` // history of deleted agents, never removed
	Trades        []Trade          `json:"trades"`
}

// StoreBackend loads and saves the full state (agents + trades).
type StoreBackend interface {
	Load() (*Store, error)
	Save(s *Store) error
}

// toPersisted converts a running agent into its persisted representation.
func (a *Agent) toPersisted() persistedAgent {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var pos *persistedPosition
	if a.pos != nil {
		pos = &persistedPosition{
			EntryTime:         a.pos.EntryTime,
			EntryBaseSmallest: a.pos.EntryBaseSmallest,
			TargetQtySmallest: a.pos.TargetQtySmallest,
		}
	}

	return persistedAgent{
		Config:            a.cfg,
		Status:            a.status,
		Position:          pos,
		ConsecutiveLosses: a.consecutiveLosses,
		TotalPnLBase:      a.totalPnLBase,
		LastAction:        a.lastAction,
	}
}

// LoadStore loads state from the given path. If the file does not exist, an
// empty store is returned.
func LoadStore(path string) (*Store, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Store{}, nil
		}
		return nil, err
	}
	var s Store
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// SaveStore writes state to disk atomically.
func SaveStore(path string, s *Store) error {
	if s == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// FileBackend persists state to a JSON file.
type FileBackend struct {
	Path string
}

// Load implements StoreBackend.
func (f *FileBackend) Load() (*Store, error) {
	return LoadStore(f.Path)
}

// Save implements StoreBackend.
func (f *FileBackend) Save(s *Store) error {
	return SaveStore(f.Path, s)
}

