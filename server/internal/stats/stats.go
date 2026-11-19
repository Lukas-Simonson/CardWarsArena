// Package stats keeps a few shared, anonymous totals for the whole site: how many games were played, how many cards,
// how many Floops. Nothing here knows who anyone is. A counter has a name and a number; there are no per-player,
// per-room or per-game records, and a game is never stored.
//
// Counting is in memory, and a Store (if there is one) is written the difference every few seconds, so a busy match
// does not cost a write per card.
package stats

import (
	"context"
	"log/slog"
	"maps"
	"sync"
	"time"
)

// Store keeps the totals between runs. AddStats adds to what is stored, so two writes never overwrite each other.
type Store interface {
	AddStats(ctx context.Context, delta map[string]int64) error
	Stats(ctx context.Context) (map[string]int64, error)
}

// Counters are the running totals. A nil *Counters is "stats are off": every method does nothing.
type Counters struct {
	mu      sync.Mutex
	totals  map[string]int64
	pending map[string]int64 // counted since the last write
	store   Store
	log     *slog.Logger
}

// New starts the totals from what the store has (st may be nil: they then start at zero and are lost on restart).
func New(st Store, log *slog.Logger) (*Counters, error) {
	c := &Counters{totals: map[string]int64{}, pending: map[string]int64{}, store: st, log: log}
	if st != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		got, err := st.Stats(ctx)
		if err != nil {
			return nil, err
		}
		maps.Copy(c.totals, got)
	}
	return c, nil
}

// Add counts n more of the named thing.
func (c *Counters) Add(name string, n int64) {
	if c == nil || n == 0 {
		return
	}
	c.mu.Lock()
	c.totals[name] += n
	c.pending[name] += n
	c.mu.Unlock()
}

// Totals is a copy of every total.
func (c *Counters) Totals() map[string]int64 {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return maps.Clone(c.totals)
}

// Kept says whether the totals survive a restart.
func (c *Counters) Kept() bool { return c != nil && c.store != nil }

// On says whether anything is being counted.
func (c *Counters) On() bool { return c != nil }

// Flush writes what was counted since the last write. On failure it is kept to be written next time.
func (c *Counters) Flush(ctx context.Context) error {
	if c == nil || c.store == nil {
		return nil
	}
	c.mu.Lock()
	delta := c.pending
	c.pending = map[string]int64{}
	c.mu.Unlock()
	if len(delta) == 0 {
		return nil
	}
	if err := c.store.AddStats(ctx, delta); err != nil {
		c.mu.Lock()
		for k, v := range delta {
			c.pending[k] += v
		}
		c.mu.Unlock()
		return err
	}
	return nil
}

// Run writes the totals every so often, and once more when ctx ends.
func (c *Counters) Run(ctx context.Context, every time.Duration) {
	if c == nil || c.store == nil {
		return
	}
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			wctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := c.Flush(wctx); err != nil && c.log != nil {
				c.log.Error("stats: final write", "err", err)
			}
			cancel()
			return
		case <-t.C:
			if err := c.Flush(ctx); err != nil && c.log != nil {
				c.log.Error("stats: write", "err", err)
			}
		}
	}
}
