package engine

import (
	"math/rand/v2"
	"testing"
)

// siloGame has Jake (player 0) active with a Silo of Truth in lane 0 and a
// known opponent hand.
func siloGame(t *testing.T, oppHand ...string) (*Game, Action) {
	t.Helper()
	g := realGame(t)
	g.State.Active = 0
	g.State.ActionsLeft = ActionsPerTurn
	g.State.Players[0].Lanes[0].Building = &Building{Instance: g.newInstance("silo-of-truth", 0), Status: Ready}
	g.State.Players[1].Hand = nil
	for _, id := range oppHand {
		g.State.Players[1].Hand = append(g.State.Players[1].Hand, g.newInstance(id, 1))
	}
	return g, Action{Kind: ActActivate, Lane: 0, Source: "building"}
}

func playPending(lane int, targets ...[]TargetRef) Action {
	return Action{Kind: ActPlayPending, Lane: lane, Targets: targets}
}

func TestSiloStealsACardThatMustBePlayedNext(t *testing.T) {
	g, silo := siloGame(t, "cool-dog")
	g.must(t, 0, silo)
	if g.State.ActionsLeft != 0 {
		t.Errorf("Silo costs 2 Actions: %d left", g.State.ActionsLeft)
	}
	if g.State.Pending == nil || g.State.Pending.Card.CardID != "cool-dog" || len(g.State.Players[1].Hand) != 0 {
		t.Fatalf("pending = %+v, opponent hand = %d", g.State.Pending, len(g.State.Players[1].Hand))
	}
	// Nothing else may happen until it is played.
	for _, a := range []Action{{Kind: ActDraw}, endMain, floop(0), activate(0, 0), {Kind: ActKeepHand}} {
		g.mustFail(t, 0, a, ErrPending)
	}
	g.mustFail(t, 1, playPending(0), ErrNotYourTurn)
	g.mustFail(t, 0, playPending(7), ErrBadLane)
}

func TestSiloCreatureIsPlayedFreeAndStaysOwnedByTheVictim(t *testing.T) {
	g, silo := siloGame(t, "cool-dog")
	g.must(t, 0, silo)
	g.must(t, 0, playPending(2))
	dog := g.State.Players[0].Lanes[2].Creature
	if dog == nil || dog.CardID != "cool-dog" || dog.Owner != 1 || dog.Status != Ready {
		t.Fatalf("stolen Creature = %+v", dog)
	}
	if g.State.Pending != nil || g.State.ActionsLeft != 0 {
		t.Error("playing it is free and ends the pending state")
	}
	// It works for its new controller (Cool Dog protects my adjacent Creatures).
	g.put(0, 1, "evil-eye")
	if g.attackable(pos{0, 1}) {
		t.Error("the stolen Cool Dog should protect its new controller's neighbours")
	}
	// And dies to its owner's discard pile.
	dog.Damage = 100
	g.settle()
	if n := len(g.State.Players[1].Discard); n != 1 {
		t.Errorf("owner's discard = %d", n)
	}
}

func TestSiloIgnoresLandscapeRequirement(t *testing.T) {
	g, silo := siloGame(t, "shark") // a SandyLands card costing 2; Jake has no SandyLands
	g.must(t, 0, silo)
	g.must(t, 0, playPending(0))
	if g.State.Players[0].Lanes[0].Creature == nil {
		t.Error("played at no cost: no Landscape needed")
	}
}

func TestSiloSpellResolvesForTheThiefAndGoesToTheOwnersDiscard(t *testing.T) {
	g, silo := siloGame(t, "gnome-snot")
	g.must(t, 0, silo)
	hand := len(g.State.Players[0].Hand)
	g.must(t, 0, playPending(0))
	if got := len(g.State.Players[0].Hand); got != hand+3 {
		t.Errorf("thief should draw 3: %d -> %d", hand, got)
	}
	if d := g.State.Players[1].Discard; len(d) != 1 || d[0].CardID != "gnome-snot" {
		t.Errorf("the Spell goes to its owner's discard: %+v", d)
	}
	if g.State.SpellsThisTurn != 1 {
		t.Error("it counts as a Spell the thief played")
	}
}

func TestSiloSpellNeedsTargetsAndIsDiscardedIfUnplayable(t *testing.T) {
	g, silo := siloGame(t, "teleport")
	g.must(t, 0, silo) // no Creatures of mine: Teleport has no legal target
	if g.State.Pending != nil {
		t.Error("an unplayable stolen card must not block the game")
	}
	if d := g.State.Players[1].Discard; len(d) != 1 || d[0].CardID != "teleport" {
		t.Errorf("an unplayable card goes to its owner's discard: %+v", d)
	}

	g, silo = siloGame(t, "teleport")
	g.put(0, 1, "evil-eye")
	g.must(t, 0, silo)
	if g.State.Pending == nil {
		t.Fatal("playable this time")
	}
	g.mustFail(t, 0, playPending(0), ErrBadTarget)
	g.must(t, 0, playPending(0, []TargetRef{creatureRef(0, 1)}, []TargetRef{laneRef(0, 3)}))
	if g.State.Players[0].Lanes[3].Creature == nil {
		t.Error("the stolen Teleport should have moved my Creature")
	}
}

func TestSiloEnterTriggerTargetsAreMine(t *testing.T) {
	g, silo := siloGame(t, "husker-worm")
	g.must(t, 0, silo)
	g.mustFail(t, 0, playPending(0), ErrBadTarget) // must choose one of MY Cornfields
	g.mustFail(t, 0, playPending(0, []TargetRef{lsRef(1, 0)}), ErrBadTarget)
	g.must(t, 0, playPending(0, []TargetRef{lsRef(0, 3)}))
	if !g.State.Players[0].Lanes[3].FaceDown {
		t.Error("the Worm flips one of its new controller's Cornfields")
	}
}

func TestSiloIgnoresAdditionalCosts(t *testing.T) {
	g, silo := siloGame(t, "field-reaper")
	g.must(t, 0, silo)
	hand := len(g.State.Players[0].Hand)
	g.mustFail(t, 0, Action{Kind: ActPlayPending, Lane: 0, CostTargets: [][]TargetRef{{handRef(0, g.State.Players[0].Hand[0].UID)}}}, ErrBadTarget)
	g.must(t, 0, playPending(0, []TargetRef{creatureRef(0, 0)}, []TargetRef{laneRef(0, 1)}))
	if len(g.State.Players[0].Hand) != hand {
		t.Error("\"at no cost\": no card is discarded")
	}
}

func TestSiloCannotReplaceAFloopedCreature(t *testing.T) {
	g, silo := siloGame(t, "cool-dog")
	for lane := 0; lane < NumLanes; lane++ {
		g.put(0, lane, "evil-eye").Status = Flooped
	}
	g.must(t, 0, silo)
	// Every lane holds a Flooped Creature: the card cannot be placed anywhere.
	if g.State.Pending != nil {
		t.Error("with nowhere to play it, the card goes to its owner's discard")
	}
	if len(g.State.Players[1].Discard) != 1 {
		t.Error("expected the stolen card in its owner's discard")
	}
}

func TestSiloWithAnEmptyHandJustSpendsTheActions(t *testing.T) {
	g, silo := siloGame(t)
	g.must(t, 0, silo)
	if g.State.Pending != nil || g.State.ActionsLeft != 0 {
		t.Errorf("pending=%v actions=%d", g.State.Pending, g.State.ActionsLeft)
	}
}

func TestSiloStealsAtRandom(t *testing.T) {
	seen := map[string]bool{}
	for seed := uint64(1); seed <= 40; seed++ {
		g, silo := siloGame(t, "cool-dog", "evil-eye", "gnome-snot")
		g.rng = *rand.NewPCG(seed, seed)
		g.must(t, 0, silo)
		in := g.State.Pending.Card
		seen[in.CardID] = true
		if len(g.State.Players[1].Hand) != 2 {
			t.Fatal("exactly one card should be taken")
		}
	}
	if len(seen) != 3 {
		t.Errorf("only saw %v", seen)
	}
}

func TestLegalActionsDuringPendingOnlyPlayTheStolenCard(t *testing.T) {
	g, silo := siloGame(t, "husker-worm")
	g.put(0, 1, "evil-eye")
	g.must(t, 0, silo)
	acts := g.LegalActions(0)
	if len(acts) == 0 {
		t.Fatal("expected plays of the stolen card")
	}
	for _, a := range acts {
		if a.Kind != ActPlayPending {
			t.Errorf("unexpected action %s", a)
		}
		if _, err := g.Clone().Apply(0, a); err != nil {
			t.Errorf("%s: %v", a, err)
		}
	}
	if n := len(g.LegalActions(1)); n != 0 {
		t.Errorf("the opponent has %d actions while I resolve the steal", n)
	}
}

func TestSiloStolenCardCanWinTheGame(t *testing.T) {
	// Field of Nightmares counts the cards left in the victim's hand, so keep
	// one there; find a seed where the Spell is the card taken.
	for seed := uint64(1); seed <= 40; seed++ {
		g, silo := siloGame(t, "field-of-nightmares", "evil-eye")
		g.State.Players[1].HP = 1
		g.rng = *rand.NewPCG(seed, seed)
		g.must(t, 0, silo)
		if g.State.Pending.Card.CardID != "field-of-nightmares" {
			continue
		}
		g.must(t, 0, playPending(0))
		if g.State.Phase != PhaseOver || g.State.Winner != 0 {
			t.Errorf("phase=%s winner=%d", g.State.Phase, g.State.Winner)
		}
		return
	}
	t.Fatal("no seed stole the Spell")
}
