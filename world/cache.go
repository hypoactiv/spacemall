package world

import (
	"fmt"
)

// Discarded nodes are stored here to be reused later, reducing GC pressure.
var nodePool []*WallTreeNode //sync.Pool has concurrency overhead?
var cacheHit, cacheReturns int

func NodeCacheStats() string {
	return fmt.Sprintf("node cache avail: %d hits: %d returns: %d", len(nodePool), cacheHit, cacheReturns)
}

func cacheNode(n *WallTreeNode) {
	n.RoomIds = nil
	nodePool = append(nodePool, n)
	cacheReturns++
}

func getNode() (n *WallTreeNode) {
	last := len(nodePool) - 1
	if last >= 0 {
		n, nodePool = nodePool[last], nodePool[:last]
		for i := range n.N {
			n.N[i] = nil
		}
		cacheHit++
		return
	}
	n = new(WallTreeNode)
	return
}

var roomCache []*Room

func cacheRoom(r *Room) {
	if len(roomCache) < 1000 {
		roomCache = append(roomCache, r)
	}
}

func getRoom() (r *Room) {
	if len(roomCache) > 0 {
		r, roomCache = roomCache[0], roomCache[1:]
	} else {
		r = new(Room)
	}
	return
}
