package domain

import (
	"reflect"
	"testing"
)

// latOffsetKm converts kilometres to degrees of latitude (≈111.19 km/°),
// letting the tests place ships at exact distances along a meridian.
func latOffsetKm(km float64) float64 { return km / 111.19 }

func testEdge() Edge {
	return Edge{ID: 1, EdgeCode: "EDGE-1", Name: "Syahbandar", Latitude: 0, Longitude: 0}
}

// TestBuildTopologyChain reproduces the PRD example C -> A -> B -> Edge:
// ships strung out every 2 km with a 3 km radio range must form a chain.
func TestBuildTopologyChain(t *testing.T) {
	nodes := []NodePosition{
		{ID: "B", Lat: latOffsetKm(2), Lng: 0},
		{ID: "A", Lat: latOffsetKm(4), Lng: 0},
		{ID: "C", Lat: latOffsetKm(6), Lng: 0},
	}
	routes := BuildTopology(nodes, []Edge{testEdge()}, 3.0)

	b := routes["B"]
	if b.Parent != "EDGE-1" || !b.ParentIsEdge || b.HopLevel != 1 || b.Status != RouteStatusRouted {
		t.Errorf("B should route directly to the edge, got %+v", b)
	}
	a := routes["A"]
	if a.Parent != "B" || a.ParentIsEdge || a.HopLevel != 2 {
		t.Errorf("A should route via B, got %+v", a)
	}
	c := routes["C"]
	if c.Parent != "A" || c.HopLevel != 3 {
		t.Errorf("C should route via A, got %+v", c)
	}
	if c.DistanceKm < 1.9 || c.DistanceKm > 2.1 {
		t.Errorf("C->A distance = %f km, want ≈2", c.DistanceKm)
	}
}

func TestBuildTopologyIsolation(t *testing.T) {
	nodes := []NodePosition{
		{ID: "NEAR", Lat: latOffsetKm(2), Lng: 0},
		{ID: "FAR", Lat: latOffsetKm(50), Lng: 0},
	}
	routes := BuildTopology(nodes, []Edge{testEdge()}, 3.0)

	if routes["NEAR"].Status != RouteStatusRouted {
		t.Errorf("NEAR should be routed, got %+v", routes["NEAR"])
	}
	far := routes["FAR"]
	if far.Status != RouteStatusIsolated || far.Parent != "" {
		t.Errorf("FAR should be isolated, got %+v", far)
	}
}

// TestBuildTopologyMaxHops: a 7-ship chain with 2 km spacing and 3 km range
// can only route 5 levels deep (MaxHops); ships 6 and 7 must be isolated.
func TestBuildTopologyMaxHops(t *testing.T) {
	var nodes []NodePosition
	ids := []string{"N1", "N2", "N3", "N4", "N5", "N6", "N7"}
	for i, id := range ids {
		nodes = append(nodes, NodePosition{ID: id, Lat: latOffsetKm(float64(2 * (i + 1))), Lng: 0})
	}
	routes := BuildTopology(nodes, []Edge{testEdge()}, 3.0)

	for i := 0; i < 5; i++ {
		r := routes[ids[i]]
		if r.Status != RouteStatusRouted || r.HopLevel != i+1 {
			t.Errorf("%s should be routed at level %d, got %+v", ids[i], i+1, r)
		}
	}
	for i := 5; i < 7; i++ {
		if routes[ids[i]].Status != RouteStatusIsolated {
			t.Errorf("%s exceeds MaxHops and should be isolated, got %+v", ids[i], routes[ids[i]])
		}
	}
}

func TestBuildTopologyDeterministic(t *testing.T) {
	nodes := []NodePosition{
		{ID: "K3", Lat: latOffsetKm(2.5), Lng: 0.001},
		{ID: "K1", Lat: latOffsetKm(2.5), Lng: -0.001},
		{ID: "K2", Lat: latOffsetKm(4.8), Lng: 0},
	}
	first := BuildTopology(nodes, []Edge{testEdge()}, 3.0)
	for i := 0; i < 10; i++ {
		again := BuildTopology(nodes, []Edge{testEdge()}, 3.0)
		if !reflect.DeepEqual(first, again) {
			t.Fatalf("topology is not deterministic: %+v vs %+v", first, again)
		}
	}
}

func TestBuildTopologyEmpty(t *testing.T) {
	if got := BuildTopology(nil, []Edge{testEdge()}, 3.0); len(got) != 0 {
		t.Errorf("no nodes must yield an empty table, got %+v", got)
	}
	// No edges: everyone is isolated.
	routes := BuildTopology([]NodePosition{{ID: "X", Lat: 0, Lng: 0}}, nil, 3.0)
	if routes["X"].Status != RouteStatusIsolated {
		t.Errorf("without edges every node is isolated, got %+v", routes["X"])
	}
}
