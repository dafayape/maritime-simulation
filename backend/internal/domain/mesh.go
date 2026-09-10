package domain

import "sort"

// Mesh routing invariants (PRD §3.3 + SRS §5C).
const (
	// MaxHops is the packet TTL: a frame that has already been transmitted
	// MaxHops times without reaching an Edge is dropped.
	MaxHops = 5
)

// Route statuses broadcast to clients and the dashboard.
const (
	RouteStatusRouted   = "routed"
	RouteStatusIsolated = "isolated"
)

// NodePosition is a live ship coordinate pulled from Redis GEO.
type NodePosition struct {
	ID  string
	Lat float64
	Lng float64
}

// RouteEntry is one row of the mesh routing table: which parent a node must
// transmit to, and how far away that parent is.
type RouteEntry struct {
	NodeID       string  `json:"node"`
	Parent       string  `json:"parent"`
	ParentIsEdge bool    `json:"parent_is_edge"`
	DistanceKm   float64 `json:"distance_km"`
	HopLevel     int     `json:"hop_level"`
	Status       string  `json:"status"`
}

// BuildTopology computes the routing DAG with BFS layering from the Virtual
// Edges outward (SRS §5C):
//
//	level 1: nodes within radio range of an Edge (parent = nearest Edge),
//	level k: unrouted nodes within range of a level k-1 node (parent =
//	         nearest such node), up to MaxHops levels.
//
// Anything left over is Isolated. Iteration order is deterministic (sorted
// node IDs, nearest-parent tie broken by parent ID) so repeated runs over an
// unchanged fleet produce an identical table.
func BuildTopology(nodes []NodePosition, edges []Edge, maxRangeKm float64) map[string]RouteEntry {
	routes := make(map[string]RouteEntry, len(nodes))
	if len(nodes) == 0 {
		return routes
	}

	sorted := make([]NodePosition, len(nodes))
	copy(sorted, nodes)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })

	positions := make(map[string]NodePosition, len(sorted))
	for _, n := range sorted {
		positions[n.ID] = n
	}

	unrouted := make(map[string]bool, len(sorted))
	for _, n := range sorted {
		unrouted[n.ID] = true
	}

	// Level 1: direct line of sight to an Edge.
	prevLevel := make([]string, 0, len(sorted))
	for _, n := range sorted {
		bestDist := 0.0
		bestEdge := ""
		found := false
		for _, e := range edges {
			d := HaversineKm(n.Lat, n.Lng, e.Latitude, e.Longitude)
			if d > maxRangeKm {
				continue
			}
			if !found || d < bestDist || (d == bestDist && e.EdgeCode < bestEdge) {
				bestDist = d
				bestEdge = e.EdgeCode
				found = true
			}
		}
		if found {
			routes[n.ID] = RouteEntry{
				NodeID:       n.ID,
				Parent:       bestEdge,
				ParentIsEdge: true,
				DistanceKm:   bestDist,
				HopLevel:     1,
				Status:       RouteStatusRouted,
			}
			delete(unrouted, n.ID)
			prevLevel = append(prevLevel, n.ID)
		}
	}

	// Levels 2..MaxHops: attach to the nearest node of the previous level.
	for level := 2; level <= MaxHops && len(unrouted) > 0 && len(prevLevel) > 0; level++ {
		currLevel := make([]string, 0)
		for _, n := range sorted {
			if !unrouted[n.ID] {
				continue
			}
			bestDist := maxRangeKm
			bestParent := ""
			found := false
			for _, pid := range prevLevel {
				p := positions[pid]
				d := HaversineKm(n.Lat, n.Lng, p.Lat, p.Lng)
				if d > maxRangeKm {
					continue
				}
				if !found || d < bestDist || (d == bestDist && pid < bestParent) {
					bestDist = d
					bestParent = pid
					found = true
				}
			}
			if found {
				routes[n.ID] = RouteEntry{
					NodeID:       n.ID,
					Parent:       bestParent,
					ParentIsEdge: false,
					DistanceKm:   bestDist,
					HopLevel:     level,
					Status:       RouteStatusRouted,
				}
				currLevel = append(currLevel, n.ID)
			}
		}
		for _, id := range currLevel {
			delete(unrouted, id)
		}
		prevLevel = currLevel
	}

	for id := range unrouted {
		routes[id] = RouteEntry{
			NodeID:   id,
			Parent:   "",
			HopLevel: 0,
			Status:   RouteStatusIsolated,
		}
	}
	return routes
}
