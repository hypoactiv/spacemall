// RouteWalkers attempt to move to their destination one tile at a time
// without colliding with other RouteWalkers

package entity

import (
	"fmt"
	"jds/game"
	"jds/game/layer"
	"jds/game/world"
	"jds/game/world/path"
	"math/rand"
)

// Definitions

const PLAN_LENGTH = 2

// A Plan is a list of Directions that this RouteWalker intends to
// take over the next PLAN_LENGTH ticks. Each Direction is the Plan
// is a 'step' of that Plan.
//
// A Plan is 'viable' if it does not result in a collision with another
// entity or wall, and each step of the plan moves closer to the route
// cursor (rc)
//
// A step of a Plan is 'viable' if it can be part of a viable plan.
// that is, a step that does not force a collision and moves closer
// to rc
//
// Once a RouteWalker computes a viable plan, it signals its intentions
// to other walkers by setting the appropriate bits in the
// RouteWalkerIntention Layer.
type Plan [PLAN_LENGTH]game.Direction

// The lowest BITWIDTH bits of the values in the RouteWalkerIntention
// layer are used to indicate if an entity will be at that tile during
// tick (now+step)%BITWIDTH where 'step' is the step in that entity's
// Plan.
//
// BITWIDTH must be greater than or equal to PLAN_LENGTH
const BITWIDTH = 8

type RouteWalker struct {
	id          world.EntityId
	w           *world.World
	l           game.Location
	sc          *layer.StackCursor
	route       path.Route
	dest        game.Location
	speed       float64
	intentions  *layer.Layer
	routeCursor game.Location
	routeStep   int
	plan        Plan
	planSet     bool // plan has been set in intention layer
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
	now := uint(w.Now())
	t.w = w
	t.id = id
	t.sc = sc
	t.routeCursor = t.l
	t.routeStep = 0
	t.route = path.NewRoute(w, t.l, t.dest)
	//fmt.Printf("spawned id:%d tick:%d loc:%v\n", t.id, now, t.l)
	for t.routeStep < PLAN_LENGTH+1 && t.routeStep < t.route.Len()-1 {
		t.routeCursor = t.routeCursor.JustStep(t.route.Direction(uint(t.routeStep)))
		t.routeStep++
	}
	t.intentions = t.w.CustomLayer("RouteWalkerIntentions")
	if t.sc.Add(t.intentions) != intentionIndex {
		panic("unexpected layer index")
	}
	for i := range t.plan {
		i := uint(i)
		t.plan[i] = game.NONE
		if t.sc.GetBit(intentionIndex, (now+i-1)%BITWIDTH) {
			panic("tried to spawn in another walker's path")
		}
		t.sc.SetBit(intentionIndex, (now+i-1)%BITWIDTH, true)
		//fmt.Println("set", t.id, now+i-1, t.sc.Cursor())
	}
	if t.sc.GetBit(intentionIndex, (now+PLAN_LENGTH-1)%BITWIDTH) {
		panic("tried to spawn in another walker's path")
	}
	t.planSet = true
	t.sc.SetBit(intentionIndex, (now+PLAN_LENGTH-1)%BITWIDTH, true)
	//fmt.Println("set", t.id, now+PLAN_LENGTH-1, t.sc.Cursor())
	if t.route.Len() > 0 {
		//ta.Add(t.w.Now()+1, t, t.l.BlockId)
		t.Act(ta)
	}
}

func (t *RouteWalker) Touched(other world.EntityId, d game.Direction) {
}

func (t *RouteWalker) HitWall(d game.Direction) {
}

func (t *RouteWalker) Act(ta *game.ThoughtAccumulator) {
	var makeplan func(uint, *Plan, game.Location, int) (rcDist int, viable bool)
	now := uint(t.w.Now())

	//fmt.Printf("*** RouteWalker Act id:%d tick:%d\n", t.id, now)
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
			//fmt.Println(plan, waits, viable)
			return
		}
		// consider possible moves from t.sc's current location. only consider staying
		// still, moves that move closer to the route cursor, and moves that don't
		// collide with other entities or walls.
		var m game.Min
		pushedDirection := game.Direction(game.NONE) // EXP take this move if we're going to be pushed
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
			// is there a wall here?
			if wallLocal[d] != 0 {
				// yes -- not viable
				continue
			}
			// is another walker going to be at rl during this or next tick?
			if intentionLocal[d]&(1<<((now+step)%BITWIDTH)) != 0 ||
				intentionLocal[d]&(1<<((now+step+1)%BITWIDTH)) != 0 {
				// yes -- not viable
				continue
			}
			t.sc.Push()
			t.sc.Step(d)
			pushedDirection = d // take this direction if we must dodge an entity trying to move to our tile
			if t.sc.Obstructed(wallIndex, rc) {
				t.sc.Pop()
				continue
			}
			if step > 0 && d.Reverse() == plan[step-1] {
				// don't backtrack
				t.sc.Pop()
				continue
			}
			if rl.MaxDistance(rc) >= curDist {
				// no progress -- not viable
				if rl.MaxDistance(rc) == curDist {
					// if there aren't any moves that decrease distance to rc,
					// then choose one that keeps it the same, so that
					// we don't just stop and block traffic.
					// here we remember directions that keep the distance the same
					// and are otherwise ok
					//
					// BUG check for obstructions?
					almostViable[d] = true
				}
				t.sc.Pop()
				continue
			}
			// d is a viable direction to move in
			plan[step] = d
			waits, viable := makeplan(step+1, plan, rc, m.Min())
			t.sc.Pop()
			if viable {
				//fmt.Println("observe", d, waits, plan)
				m.Observe(*plan, waits)
				if waits == 0 {
					// no better plan possible
					break
				}
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
						//fmt.Println("observe almost viable", d, waits, plan)
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
			if t.sc.GetBit(intentionIndex, (now+step+1)%BITWIDTH) {
				// some other entity wants our tile, but we have no valid move.
				// unavoidable collision -> avoid this plan
				if step == 0 {
					fmt.Println("pushed! panic direction", pushedDirection, t.id)
				}
				plan[step] = pushedDirection
				waits, viable := makeplan(step+1, plan, rc, best)
				return waits + 2, viable // getting pushed is worth 2 waits since we lose progress to rc
			} else {
				// no other entit wants our tile, so wait here until congestion clears out
				plan[step] = game.NONE
				return makeplan(step+1, plan, rc, best)
			}
		}
		return
	}

	var tookStep bool
	// remove intention to follow old plan
	//fmt.Println("current plan", t.plan)
	t.sc.Push()
	for step, d := range t.plan {
		step := uint(step)
		//fmt.Println("unset", t.sc.Cursor(), (now+step-1)%BITWIDTH)
		if t.planSet && !t.sc.GetBit(intentionIndex, (now+step-1)%BITWIDTH) {
			//fmt.Println(now+step-1, t.plan, t.dest, t.sc.Cursor())
			panic("intention changed unexpectedly")
		}
		t.sc.SetBit(intentionIndex, (now+step-1)%BITWIDTH, false)
		//fmt.Println("unset", t.id, now+step-1, t.sc.Cursor())
		//fmt.Println("clear", t.sc.Cursor())
		t.sc.Step(d)
		if t.planSet && d != game.NONE && !t.sc.GetBit(intentionIndex, (now+step-1)%BITWIDTH) {
			//fmt.Println(now+step-1, t.plan, t.dest, t.sc.Cursor())
			panic("intention changed unexpectedly")
		}
		t.sc.SetBit(intentionIndex, (now+step-1)%BITWIDTH, false)
		//fmt.Println("unset", t.id, now+step-1, t.sc.Cursor())
	}
	//fmt.Println("clear", t.sc.Cursor())
	if t.planSet && !t.sc.GetBit(intentionIndex, (now+PLAN_LENGTH-1)%BITWIDTH) {
		//fmt.Println(PLAN_LENGTH, t.plan, t.dest, t.sc.Cursor())
		panic("intention changed unexpectedly")
	}
	t.sc.SetBit(intentionIndex, (now+PLAN_LENGTH-1)%BITWIDTH, false)
	//fmt.Println("unset", t.id, now+PLAN_LENGTH-1, t.sc.Cursor())
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
		panic("collided!")
		//fmt.Println("no step")
	}
	// advance route cursor
	for i := 0; i < 2; i++ {
		if t.routeStep < t.route.Len() {
			newrc := t.routeCursor.JustStep(t.route.Direction(uint(t.routeStep)))
			if !t.sc.Obstructed(wallIndex, newrc) {
				t.routeCursor = newrc
				t.routeStep++
			} else {
				break
			}
		}
	}
	// make a new plan
	_, viable := makeplan(0, &t.plan, t.routeCursor, t.route.Len())
	if !viable {
		// TODO there is, or will be, some forced collision. what should be done?
		panic("no viable path!")
	}
	// TODO remove sanity check
	t.sc.Push()
	for step, d := range t.plan {
		step := uint(step)
		if t.sc.GetBit(intentionIndex, (now+step)%BITWIDTH) {
			//fmt.Println(step, t.plan)
			panic("makeplan returned path with collision")
		}
		t.sc.Step(d)
		if t.sc.GetBit(intentionIndex, (now+step)%BITWIDTH) {
			//fmt.Println(step, t.plan, t.dest, t.sc.Cursor())
			panic("makeplan returned path with collision")
		}
	}
	if t.sc.GetBit(intentionIndex, (now+PLAN_LENGTH)%BITWIDTH) {
		//fmt.Println(step, t.plan, t.dest, t.sc.Cursor())
		panic("makeplan returned path with collision")
	}
	t.sc.Pop()
	//fmt.Printf("made a plan with %d waits %v\n", waits, t.plan)
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
		//fmt.Println("set", t.id, now+step, t.sc.Cursor())
		t.sc.Step(d)
		if d != game.NONE && t.sc.GetBit(intentionIndex, (now+step)%BITWIDTH) {
			//fmt.Println(step)
			panic("intention bit already set")
		}
		t.sc.SetBit(intentionIndex, (now+step)%BITWIDTH, true)
		//fmt.Println("set", t.id, now+step, t.sc.Cursor())
	}
	if t.sc.GetBit(intentionIndex, (now+PLAN_LENGTH)%BITWIDTH) {
		panic("intention bit already set")
	}
	t.sc.SetBit(intentionIndex, (now+PLAN_LENGTH)%BITWIDTH, true)
	//fmt.Println("set", t.id, now+PLAN_LENGTH, t.sc.Cursor())
	t.sc.Pop()
	t.planSet = true
	//fmt.Println("local intentions", t.sc.Get(intentionIndex))
	//fmt.Println("nearby intentions", t.sc.Look(intentionIndex))
	ta.Add(game.Ticks(now+1), t, t.l.BlockId)
	// TODO remove sanity checks
	if t.l != t.sc.Cursor() || !t.sc.GetBit(intentionIndex, (now)%BITWIDTH) {
		panic("asdf")
	}
	if t.l == t.dest {
		//fmt.Println("GOAL REACHED")
	}
}
