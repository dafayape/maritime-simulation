package domain

import "testing"

func validPacket() TransmitPacket {
	return TransmitPacket{
		Event:            "node:transmit",
		PacketID:         "pkt-9912A",
		OriginNode:       "KPL-001",
		TargetParent:     "KPL-002",
		HopCount:         1,
		RoutingPath:      []string{"KPL-001"},
		BinaryPayloadB64: "AAEC",
	}
}

func TestTransmitPacketValidate(t *testing.T) {
	p := validPacket()
	if err := p.Validate(); err != nil {
		t.Fatalf("valid packet rejected: %v", err)
	}

	mutations := []func(*TransmitPacket){
		func(p *TransmitPacket) { p.PacketID = "" },
		func(p *TransmitPacket) { p.OriginNode = "" },
		func(p *TransmitPacket) { p.TargetParent = "" },
		func(p *TransmitPacket) { p.HopCount = 0 },
		func(p *TransmitPacket) { p.RoutingPath = nil },
		func(p *TransmitPacket) { p.RoutingPath = []string{"a", "b", "c", "d", "e", "f", "g"} },
		func(p *TransmitPacket) { p.BinaryPayloadB64 = "" },
		func(p *TransmitPacket) { p.RoutingPath = []string{""} },
	}
	for i, mutate := range mutations {
		bad := validPacket()
		mutate(&bad)
		if err := bad.Validate(); err == nil {
			t.Errorf("mutation %d should fail validation", i)
		}
	}
}

func TestTransmitterAndPath(t *testing.T) {
	p := validPacket()
	p.RoutingPath = []string{"KPL-001", "KPL-002"}
	if got := p.Transmitter(); got != "KPL-002" {
		t.Errorf("Transmitter() = %q, want KPL-002", got)
	}
	if got := p.PathString("EDGE-1"); got != "KPL-001->KPL-002->EDGE-1" {
		t.Errorf("PathString() = %q", got)
	}
}
