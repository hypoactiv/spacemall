package entity

import (
	"fmt"
	"jds/game"
	"jds/game/layer"
	"jds/game/world"
	"jds/game/world/path"
	"math/rand"
)

const PLAN_LENGTH = 4
const BITWIDTH = 8

type Plan [PLAN_LENGTH]game.Direction

type RouteWalker struct {
	id         world.EntityId
	w          *world.World
	l          game.Location
	sc         *layer.StackCursor
	route      path.Route
	dest       game.Location
	speed      float64
	intentions *layer.Layer
	routeLoc   game.Location
	routeStep  int
	plan       Plan
	planSet    bool // plan has been set in intention layer
}

const (
	entityIndex    = 0
	wallIndex      = 1
	intentionIndex = 2
)

func NewRouteWalker(l game.Location, dest game.Location) *RouteWalker {
	return &RouteWalker{
		l:     l,
		dest:  dest,
		speed: rand.Float64()*0.8 + 0.2,
	}
}

func (t *RouteWalker) Location() game.Location {
	return t.l
}

func (t *RouteWalker) Spawned(ta *game.ThoughtAccumulator, id world.EntityId, w *world.World, sc *layer.StackCursor) {
	t.w = w
	t.id = id
	t.sc = sc
	t.routeLoc = t.l
	t.routeStep = 0
	t.route = path.NewRoute(w, t.l, t.dest)
	for t.routeStep < PLAN_LENGTH+1 && t.routeStep < t.route.Len()-1 {
		t.routeLoc = t.routeLoc.JustStep(t.route.Direction(uint(t.routeStep)))
		t.routeStep++
	}
	for i := range t.plan {
		t.plan[i] = game.NONE
	}
	t.intentions = t.w.CustomLayer("RouteWalkerIntentions")
	if t.sc.Add(t.intentions) != intentionIndex {
		panic("unexpected layer index")
	}
	if t.route.Len() > 0 {
		ta.Add(t.w.Now()+1, t, t.l.BlockId)
	}
}

func (t *RouteWalker) Touched(other world.EntityId, d game.Direction) {
}

func (t *RouteWalker) HitWall(d game.Direction) {
}

func (t *RouteWalker) Act(ta *game.ThoughtAccumulator) {
	var makeplan func(uint, *Plan, game.Location, int) (rcDist int, viable bool)
	now := uint(t.w.Now())

	// a plan is 'feasible' if it does not result in a collision with another
	// entity or wall, and each step moves closer to the route cursor
	//
	// a step of a plan is 'viable' if it can be part of a feasible plan
	makeplan = func(step uint, plan *Plan, rc game.Location, best int) (waits int, viable bool) {
		if step >= PLAN_LENGTH {
			// end of plan.
			//rcDist = t.sc.Cursor().MaxDistance(rc)
			for _, d := range plan {
				if d == game.NONE {
					waits++
				}
			}
			viable = !t.sc.Obstructed(wallIndex, rc)
			fmt.Println(plan, waits, viable)
			return
		}
		// consider possible moves from t.sc's current location. only consider staying
		// still, moves that move closer to the route cursor, and moves that don't
		// collide with other entities or walls.
		var m game.Min
		curDist := t.sc.Cursor().MaxDistance(rc)
		//plan[step] = game.NONE
		//rcDist, viable = makeplan(step+1, plan, rc, best)
		//fmt.Println("observe none", rcDist)
		//m.Observe(game.NONE, rcDist)
		wallLocal := t.sc.Look(wallIndex)
		intentionLocal := t.sc.Look(intentionIndex)
		almostViable := [8]bool{}
		for d, rl := range t.sc.Cursor().Neighborhood() {
			d := game.Direction(d)
			// move close to rc?
			if step > 0 && d.Reverse() == plan[step-1] {
				// don't backtrack
				continue
			}
			// is there a wall here?
			if wallLocal[d] != 0 {
				// yes -- not viable
				continue
			}
			// is another entity going to move here?
			fmt.Println(intentionLocal)
			if intentionLocal[d]&(1<<((now+step)%BITWIDTH)) != 0 ||
				intentionLocal[d]&(1<<((now+step+1)%BITWIDTH)) != 0 {
				fmt.Println(intentionLocal)
				fmt.Println("direction", d, "would collide", step)
				// yes -- not viable
				continue
			}
			if rl.MaxDistance(rc) >= curDist {
				// no progress -- not viable
				if rl.MaxDistance(rc) == curDist {
					almostViable[d] = true
				}
				continue
			}
			// d is a viable direction to move in
			plan[step] = d
			t.sc.Push()
			t.sc.Step(d)
			if t.sc.Obstructed(wallIndex, rc) {
				t.sc.Pop()
				continue
			}
			waits, viable := makeplan(step+1, plan, rc, m.Min())
			t.sc.Pop()
			if viable {
				fmt.Println("observe", d, waits, plan)
				m.Observe(*plan, waits)
			}
		}
		if !m.Feasible() {
			for d, v := range almostViable {
				d := game.Direction(d)
				if v {
					plan[step] = d
					t.sc.Push()
					t.sc.Step(d)
					if t.sc.Obstructed(wallIndex, rc) {
						t.sc.Pop()
						continue
					}
					waits, viable := makeplan(step+1, plan, rc, m.Min())
					t.sc.Pop()
					if viable {
						fmt.Println("observe almost viable", d, waits, plan)
						m.Observe(*plan, waits)
					}
				}
			}
		}
		if m.Feasible() {
			// found new best path
			viable = true
			waits = m.Min()
			for i, d := range m.Argmin().(Plan) {
				plan[i] = d
			}
			//makeplan(step+1, plan, rc, waits)
		} else {
			// Unable to find a viable direction. Wait 1 tick and try again
			plan[step] = game.NONE
			if t.sc.GetBit(intentionIndex, (now+step+1)%BITWIDTH) {
				fmt.Println("pushed!")
			}
			return makeplan(step+1, plan, rc, best)
		}
		return
	}

	var tookStep bool
	// remove intention to follow old plan
	t.sc.Push()
	for step, d := range t.plan {
		step := uint(step)
		//fmt.Println("unset", t.sc.Cursor(), (now+step-1)%BITWIDTH)
		if t.planSet && !t.sc.GetBit(intentionIndex, (now+step-1)%BITWIDTH) {
			panic("intention changed unexpectedly")
		}
		t.sc.SetBit(intentionIndex, (now+step-1)%BITWIDTH, false)
		fmt.Println("clear", t.sc.Cursor())
		t.sc.Step(d)
		t.sc.SetBit(intentionIndex, (now+step-1)%BITWIDTH, false)
	}
	fmt.Println("clear", t.sc.Cursor())
	t.sc.SetBit(intentionIndex, (now+PLAN_LENGTH-1)%BITWIDTH, false)
	t.sc.Pop()
	t.planSet = false
	// TODO remove sanity check
	//f := t.intentions.DeepSearchNonZero()
	//if len(f) > 0 {
	//	panic("did not clear all flags")
	//}
	// step according to plan
	t.l, tookStep = t.w.StepEntity(t.id, t, t.sc, t.plan[0])
	if !tookStep {
		fmt.Println("no step")
	}
	// advance route cursor
	for i := 0; i < 2; i++ {
		if t.routeStep < t.route.Len() {
			newrc := t.routeLoc.JustStep(t.route.Direction(uint(t.routeStep)))
			if !t.sc.Obstructed(wallIndex, newrc) {
				t.routeLoc = newrc
				t.routeStep++
			} else {
				break
			}
		}
	}
	// make a new plan
	waits, _ := makeplan(0, &t.plan, t.routeLoc, t.route.Len())
	// TODO remove sanity check
	t.sc.Push()
	for step, d := range t.plan {
		step := uint(step)
		if t.sc.GetBit(intentionIndex, (now+step)%BITWIDTH) {
			fmt.Println(step, t.plan)
			panic("makeplan returned path with collision")
		}
		t.sc.Step(d)
		if t.sc.GetBit(intentionIndex, (now+step)%BITWIDTH) {
			fmt.Println(step, t.plan, t.dest, t.sc.Cursor())
			panic("makeplan returned path with collision")
		}
	}
	t.sc.Pop()
	fmt.Printf("made a plan with %d waits %v\n", waits, t.plan)
	// signal our intention to follow this plan
	// BUG intention layer needs to be used to ensure both current and next
	// positions are claimed, not just destination
	//
	// e.g. for an entity walking to the right:
	//
	// t X----->
	// 0 11....
	// 1 .22...
	// 2 ..44..
	// 3 ...88.
	//
	// intention layer flags are the sum of above
	//   1 3 6 12 8
	t.sc.Push()
	for step, d := range t.plan {
		step := uint(step)
		//fmt.Println("set", t.sc.Cursor(), (now+step)%BITWIDTH)
		if t.sc.GetBit(intentionIndex, (now+step)%BITWIDTH) {
			panic("intention bit already set")
		}
		t.sc.SetBit(intentionIndex, (now+step)%BITWIDTH, true)
		t.sc.Step(d)
		if d != game.NONE && t.sc.GetBit(intentionIndex, (now+step)%BITWIDTH) {
			fmt.Println(step)
			panic("intention bit already set")
		}
		t.sc.SetBit(intentionIndex, (now+step)%BITWIDTH, true)
	}
	if t.sc.GetBit(intentionIndex, (now+PLAN_LENGTH)%BITWIDTH) {
		panic("intention bit already set")
	}
	t.sc.SetBit(intentionIndex, (now+PLAN_LENGTH)%BITWIDTH, true)
	t.sc.Pop()
	t.planSet = true
	fmt.Println("local intentions", t.sc.Get(intentionIndex))
	fmt.Println("nearby intentions", t.sc.Look(intentionIndex))
	ta.Add(game.Ticks(now+1), t, t.l.BlockId)
	// TODO remove sanity checks
	if t.l != t.sc.Cursor() || !t.sc.GetBit(intentionIndex, (now)%BITWIDTH) {
		panic("asdf")
	}
	if t.l == t.dest {
		fmt.Println("GOAL REACHED")
	}
}
