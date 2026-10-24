package engine

import "testing"

func TestDebugHelpers(t *testing.T) {
	g := realGame(t)
	in, err := g.Give(1, "teleport")
	if err != nil || in.Owner != 1 || g.handIndex(1, in.UID) < 0 {
		t.Fatalf("Give: %+v %v", in, err)
	}
	if _, err := g.Give(1, "nope"); err == nil {
		t.Error("unknown card should fail")
	}
	if err := g.Put(1, 2, "the-pig"); err != nil || g.State.Players[1].Lanes[2].Creature == nil {
		t.Errorf("Put creature: %v", err)
	}
	if err := g.Put(1, 2, "blood-fortress"); err != nil || g.State.Players[1].Lanes[2].Building == nil {
		t.Errorf("Put building: %v", err)
	}
	if err := g.Put(1, 2, "teleport"); err == nil {
		t.Error("a Spell cannot be put in a Lane")
	}
	if err := g.Put(1, 9, "the-pig"); err != ErrBadLane {
		t.Errorf("bad lane: %v", err)
	}
	if err := g.Clear(1, 2); err != nil || g.State.Players[1].Lanes[2].Creature != nil || g.State.Players[1].Lanes[2].Building != nil {
		t.Errorf("Clear: %v", err)
	}
	if len(g.State.Players[1].Discard) != 2 {
		t.Errorf("cleared cards go to the discard pile: %d", len(g.State.Players[1].Discard))
	}
	g.SetHP(0, 0)
	if g.State.Phase != PhaseOver {
		t.Error("reducing HP to 0 ends the game")
	}
	names := g.Instances()
	if names[in.UID] != "teleport" {
		t.Errorf("Instances: %v", names[in.UID])
	}
}
