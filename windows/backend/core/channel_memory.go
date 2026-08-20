package core

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// ChannelRecord is the history of a single channel (protocol/endpoint/IP), the
// "remember" link of the adaptive loop. Keys are deterministic and MUST NOT
// contain secrets (no private_key / password) — only type+endpoint+port+version.
type ChannelRecord struct {
	Key       string    `json:"key"`
	OK        int       `json:"ok"`   // consecutive successful checks
	Fail      int       `json:"fail"` // consecutive failures
	LastOK    time.Time `json:"lastOk"`
	LastFail  time.Time `json:"lastFail"`
	TotalOK   int       `json:"totalOk"`   // lifetime successes
	TotalFail int       `json:"totalFail"` // lifetime failures
}

// ChannelMemory stores per-channel history (bandit-style preference) and
// persists it to a JSON file next to the executable. Safe for concurrent use:
// all mutations happen under the mutex; Save() is called by the owner.
type ChannelMemory struct {
	mu      sync.Mutex
	recs    map[string]*ChannelRecord
	path    string
	maxKeys int
}

// NewChannelMemory builds an in-memory store with an optional file path.
func NewChannelMemory(path string) *ChannelMemory {
	return &ChannelMemory{
		recs:    make(map[string]*ChannelRecord),
		path:    path,
		maxKeys: 256,
	}
}

// Record updates the history for a channel. ok=true on a successful probe.
func (m *ChannelMemory) Record(key string, ok bool) {
	if key == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	rec := m.recs[key]
	if rec == nil {
		rec = &ChannelRecord{Key: key}
		m.recs[key] = rec
	}
	now := time.Now()
	if ok {
		rec.OK++
		rec.Fail = 0
		rec.LastOK = now
		rec.TotalOK++
	} else {
		rec.Fail++
		rec.OK = 0
		rec.LastFail = now
		rec.TotalFail++
	}
}

// Score returns a preference value for a channel: higher is better. It combines
// reliability (fewer consecutive failures) with freshness (how recently it last
// worked). A channel that failed recently is scored down so it is not picked
// first on the next reload.
func (m *ChannelMemory) Score(key string) float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec := m.recs[key]
	if rec == nil {
		// Unknown channel: neutral score (slightly above a badly-failing one).
		return 0.5
	}
	return m.scoreLocked(key, rec)
}

// Best returns the channel with the highest score among candidates, or "" if
// the candidate list is empty. Unknown candidates get a neutral score.
func (m *ChannelMemory) Best(candidates []string) string {
	return m.BestExcept(candidates, "")
}

// BestExcept selects the highest-scoring candidate other than excluded. It is
// used immediately after a failure so the controller explores another
// validated channel instead of reloading the same endpoint.
func (m *ChannelMemory) BestExcept(candidates []string, excluded string) string {
	bestKey := ""
	bestScore := -1.0
	for _, k := range candidates {
		if k == excluded {
			continue
		}
		s := m.Score(k)
		if s > bestScore {
			bestScore = s
			bestKey = k
		}
	}
	return bestKey
}

// Load reads the persisted file into memory.
func (m *ChannelMemory) Load() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.path == "" {
		return nil
	}
	data, err := os.ReadFile(m.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // first run, nothing to load
		}
		return err
	}
	var recs map[string]*ChannelRecord
	if err := json.Unmarshal(data, &recs); err != nil {
		return fmt.Errorf("parse channel memory: %w", err)
	}
	m.recs = recs
	if m.recs == nil {
		m.recs = make(map[string]*ChannelRecord)
	}
	return nil
}

// Save writes the store to disk (atomically via a temp+rename).
func (m *ChannelMemory) Save() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.path == "" {
		return nil
	}
	data, err := json.MarshalIndent(m.recs, "", "  ")
	if err != nil {
		return err
	}
	tmp := m.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, m.path)
}

// Prune removes records whose keys are no longer present in the config.
func (m *ChannelMemory) Prune(validKeys []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	valid := make(map[string]bool, len(validKeys))
	for _, k := range validKeys {
		valid[k] = true
	}
	for k := range m.recs {
		if !valid[k] {
			delete(m.recs, k)
		}
	}
}

// EnforceCap trims the store to maxKeys (LRU by last activity) so it never
// grows unbounded as protocol×endpoint×I1 combinations accumulate.
func (m *ChannelMemory) EnforceCap() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.recs) <= m.maxKeys {
		return
	}
	type kv struct {
		key string
		at  time.Time
	}
	all := make([]kv, 0, len(m.recs))
	for k, r := range m.recs {
		at := r.LastOK
		if r.LastFail.After(at) {
			at = r.LastFail
		}
		all = append(all, kv{k, at})
	}
	sort.Slice(all, func(i, j int) bool { return all[i].at.After(all[j].at) })
	for _, item := range all[m.maxKeys:] {
		delete(m.recs, item.key)
	}
}

// ChannelMemorySummary is a compact view for Diagnostics() / the UI.
type ChannelMemorySummary struct {
	Total     int            `json:"total"`     // channels tracked
	BestKey   string         `json:"bestKey"`   // highest-scoring known channel
	BestScore float64        `json:"bestScore"` // its score
	Top       []ChannelScore `json:"top"`       // top N by score, for the UI
}

// ChannelScore is one channel's score for display.
type ChannelScore struct {
	Key   string  `json:"key"`
	Score float64 `json:"score"`
	Fail  int     `json:"fail"`
	OK    int     `json:"ok"`
}

// Summary returns a snapshot for diagnostics.
func (m *ChannelMemory) Summary(topN int) ChannelMemorySummary {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := ChannelMemorySummary{Total: len(m.recs)}
	all := make([]ChannelScore, 0, len(m.recs))
	for k, r := range m.recs {
		all = append(all, ChannelScore{Key: k, Score: m.scoreLocked(k, r), Fail: r.Fail, OK: r.OK})
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Score > all[j].Score })
	if len(all) > 0 {
		s.BestKey = all[0].Key
		s.BestScore = all[0].Score
	}
	if len(all) > topN {
		all = all[:topN]
	}
	s.Top = all
	return s
}

// scoreLocked computes a channel's score (caller holds m.mu). It is kept in a
// separate helper so both Score and Summary share the same formula.
func (m *ChannelMemory) scoreLocked(key string, rec *ChannelRecord) float64 {
	if rec == nil {
		return 0.5
	}
	reliability := 1.0 / (1.0 + float64(rec.Fail))
	const freshnessT = 20 * time.Minute
	age := time.Since(rec.LastOK)
	if rec.TotalOK == 0 {
		age = time.Since(rec.LastFail)
	}
	freshness := math.Exp(-float64(age) / float64(freshnessT))
	if time.Since(rec.LastFail) < 5*time.Minute && rec.Fail > 0 {
		freshness *= 0.5
	}
	return reliability * freshness
}

// DefaultChannelMemoryPath returns the persistence path next to the executable.
func DefaultChannelMemoryPath() string {
	if exePath, err := os.Executable(); err == nil {
		return filepath.Join(filepath.Dir(exePath), "channel-memory.json")
	}
	return "channel-memory.json"
}
