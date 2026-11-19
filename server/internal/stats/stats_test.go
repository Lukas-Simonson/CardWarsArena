package stats

import (
	"context"
	"errors"
	"testing"
)

// memStore is a Store in memory; fail makes writes fail.
type memStore struct {
	m    map[string]int64
	fail bool
}

func (s *memStore) AddStats(_ context.Context, d map[string]int64) error {
	if s.fail {
		return errors.New("down")
	}
	for k, v := range d {
		s.m[k] += v
	}
	return nil
}
func (s *memStore) Stats(context.Context) (map[string]int64, error) {
	out := map[string]int64{}
	for k, v := range s.m {
		out[k] = v
	}
	return out, nil
}

func TestCountingStartsFromTheStoreAndWritesOnlyTheDifference(t *testing.T) {
	st := &memStore{m: map[string]int64{"floops": 10}}
	c, _ := New(st, nil)
	c.Add("floops", 2)
	c.Add("fights", 1)
	if c.Totals()["floops"] != 12 {
		t.Errorf("totals %v", c.Totals())
	}
	if err := c.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	if st.m["floops"] != 12 || st.m["fights"] != 1 {
		t.Errorf("stored %v", st.m)
	}
	c.Add("floops", 1)
	c.Flush(context.Background())
	c.Flush(context.Background()) // nothing new: nothing written, nothing doubled
	if st.m["floops"] != 13 {
		t.Errorf("stored %v", st.m)
	}
}

func TestAFailedWriteIsKeptForTheNextOne(t *testing.T) {
	st := &memStore{m: map[string]int64{}}
	c, _ := New(st, nil)
	c.Add("fights", 3)
	st.fail = true
	if c.Flush(context.Background()) == nil {
		t.Fatal("expected the failure")
	}
	st.fail = false
	c.Add("fights", 1)
	c.Flush(context.Background())
	if st.m["fights"] != 4 {
		t.Errorf("stored %v", st.m)
	}
}

func TestWithoutAStoreTheTotalsAreInMemoryOnly(t *testing.T) {
	c, _ := New(nil, nil)
	c.Add("fights", 1)
	if c.Kept() || c.Totals()["fights"] != 1 || c.Flush(context.Background()) != nil {
		t.Error("memory only")
	}
}

func TestAnOffCounterDoesNothing(t *testing.T) {
	var c *Counters
	c.Add("fights", 1)
	if c.On() || c.Kept() || c.Totals() != nil || c.Flush(context.Background()) != nil {
		t.Error("a nil Counters is off")
	}
}

func TestDecksAreBucketedWithoutNamingAnyone(t *testing.T) {
	if DeckBucket("Finn Vs Jake", "finn", "x") != "finn" || DeckBucket("", "finn", "x") != "custom" || DeckBucket("Set", "", "Set Deck") != "Set Deck" {
		t.Error("buckets")
	}
}
