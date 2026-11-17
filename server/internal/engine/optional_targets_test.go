package engine

import (
	"slices"
	"testing"
)

// reclaimActions are the ways to play Reclaim Landscape that the game lists, as (flip, move, to) lengths.
func reclaimActions(t *testing.T, g *Game) []Action {
	t.Helper()
	uid := g.give(0, "reclaim-landscape")
	var out []Action
	for _, a := range g.LegalActions(0) {
		if a.Kind == ActPlayCard && a.Card == uid {
			out = append(out, a)
		}
	}
	return out
}

func slotLens(a Action) [3]int {
	var n [3]int
	for i := 0; i < 3 && i < len(a.Targets); i++ {
		n[i] = len(a.Targets[i])
	}
	return n
}

func TestReclaimLandscapeAsksForALaneOnlyAfterABuildingIsChosen(t *testing.T) {
	g := realGame(t)
	g.State.Active = 0
	g.State.Players[0].Lanes[1].FaceDown = true
	g.State.Players[0].Lanes[0].Building = &Building{Instance: Instance{UID: 800, CardID: "blood-fortress"}}

	seen := map[[3]int]int{}
	for _, a := range reclaimActions(t, g) {
		n := slotLens(a)
		seen[n]++
		if n[1] == 0 && n[2] != 0 {
			t.Errorf("a destination Lane without a Building to move: %v", a.Targets)
		}
		if n[1] == 1 && n[2] != 1 {
			t.Errorf("a Building to move without a destination Lane: %v", a.Targets)
		}
	}
	// flip only, move only, both and neither are all there, and nothing else
	for _, want := range [][3]int{{0, 0, 0}, {1, 0, 0}, {0, 1, 1}, {1, 1, 1}} {
		if seen[want] == 0 {
			t.Errorf("no action of shape %v among %v", want, seen)
		}
	}
	if len(seen) != 4 {
		t.Errorf("shapes %v", seen)
	}
}

func TestReclaimLandscapeRefusesALaneWithoutABuildingToMove(t *testing.T) {
	g := realGame(t)
	g.State.Active = 0
	g.castFails(t, 0, "reclaim-landscape", ErrBadTarget, nil, nil, []TargetRef{laneRef(0, 2)})
	g.State.Players[0].Lanes[0].Building = &Building{Instance: Instance{UID: 800, CardID: "blood-fortress"}}
	g.castFails(t, 0, "reclaim-landscape", ErrBadTarget, nil, []TargetRef{{Kind: "building", Player: 0, Lane: 0}}, nil) // moved, but nowhere
	// the old way of skipping still works
	g.cast(t, 0, "reclaim-landscape")
	g.State.ActionsLeft = 2
	g.cast(t, 0, "reclaim-landscape", nil, []TargetRef{{Kind: "building", Player: 0, Lane: 0}}, []TargetRef{laneRef(0, 3)})
	if g.State.Players[0].Lanes[3].Building == nil || g.State.Players[0].Lanes[0].Building != nil {
		t.Error("the Building should have moved to lane 4")
	}
}

func TestSlotsThatHaveNothingToChooseAreSkipped(t *testing.T) {
	g := realGame(t)
	g.State.Active = 0
	// No face-down Landscape and no Building: the only way to play it is to choose nothing.
	acts := reclaimActions(t, g)
	if len(acts) != 1 || slotLens(acts[0]) != [3]int{} {
		t.Fatalf("actions %v", acts)
	}
	// A Building but every Lane has one: it can be chosen, and there is nowhere to put it.
	for lane := range g.State.Players[0].Lanes {
		g.State.Players[0].Lanes[lane].Building = &Building{Instance: Instance{UID: 800 + lane, CardID: "blood-fortress"}}
	}
	for _, a := range reclaimActions(t, g) {
		if n := slotLens(a); n[2] != 0 {
			t.Errorf("a Lane to move to with none free: %v", a.Targets)
		}
	}
}

func TestReclaimLandscapeSaysWhatEachChoiceAndEachSkipDoes(t *testing.T) {
	g := realGame(t)
	g.State.Active = 0
	a := reclaimActions(t, g)[0]
	if !slices.Equal(a.Labels, []string{"Flip", "Move", "Move to"}) {
		t.Errorf("labels %v", a.Labels)
	}
	if !slices.Equal(a.SkipLabels, []string{"Don't flip a Landscape", "Don't move a Building", ""}) {
		t.Errorf("skip labels %v", a.SkipLabels)
	}
}
