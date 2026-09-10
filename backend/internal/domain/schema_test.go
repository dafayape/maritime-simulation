package domain

import (
	"math"
	"testing"

	"github.com/vmihailenco/msgpack/v5"
)

func sampleFields() []SchemaField {
	return []SchemaField{
		{Name: "lat", Type: "float32"},
		{Name: "lng", Type: "float32"},
		{Name: "berat_kg", Type: "uint16"},
		{Name: "jenis_ikan", Type: "string_10"},
		{Name: "fresh", Type: "bool"},
	}
}

func TestValidateSchemaFields(t *testing.T) {
	if err := ValidateSchemaFields(sampleFields()); err != nil {
		t.Fatalf("valid schema rejected: %v", err)
	}
	if err := ValidateSchemaFields(nil); err == nil {
		t.Error("empty schema must be rejected")
	}
	if err := ValidateSchemaFields([]SchemaField{{Name: "a", Type: "float32"}, {Name: "a", Type: "bool"}}); err == nil {
		t.Error("duplicate names must be rejected")
	}
	if err := ValidateSchemaFields([]SchemaField{{Name: "1bad", Type: "float32"}}); err == nil {
		t.Error("invalid field name must be rejected")
	}
	if err := ValidateSchemaFields([]SchemaField{{Name: "x", Type: "varchar"}}); err == nil {
		t.Error("unknown type must be rejected")
	}
}

func TestFieldByteSize(t *testing.T) {
	cases := map[string]int{
		"float32": 4, "float64": 8, "uint16": 2, "bool": 1, "string_10": 10, "string_242": 242,
	}
	for typ, want := range cases {
		got, err := FieldByteSize(typ)
		if err != nil || got != want {
			t.Errorf("FieldByteSize(%q) = %d, %v; want %d", typ, got, err, want)
		}
	}
	for _, bad := range []string{"string_0", "string_243", "varchar", "string_"} {
		if _, err := FieldByteSize(bad); err == nil {
			t.Errorf("FieldByteSize(%q) should fail", bad)
		}
	}
}

func TestEstimatePackedBytes(t *testing.T) {
	fields := sampleFields()
	est := EstimatePackedBytes(fields)
	// Data alone: 4+4+2+10+1 = 21 bytes; estimate must add key overhead.
	if est <= 21 {
		t.Errorf("estimate %d should exceed raw data size 21", est)
	}
	// A realistic fishing payload must fit the SF7 window.
	if est > MaxPayloadBytes(7) {
		t.Errorf("estimate %d unexpectedly exceeds SF7 limit", est)
	}
}

func TestDecodePayloadRoundtrip(t *testing.T) {
	raw, err := msgpack.Marshal(map[string]any{
		"lat":        float32(-6.5),
		"lng":        float32(106.25),
		"berat_kg":   uint16(120),
		"jenis_ikan": "tuna",
		"fresh":      true,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	decoded, problems := DecodePayload(raw, sampleFields())
	if len(problems) != 0 {
		t.Fatalf("unexpected problems: %v", problems)
	}
	if lat, ok := decoded["lat"].(float64); !ok || math.Abs(lat+6.5) > 1e-6 {
		t.Errorf("lat = %v, want -6.5", decoded["lat"])
	}
	if w, ok := decoded["berat_kg"].(int64); !ok || w != 120 {
		t.Errorf("berat_kg = %v, want 120", decoded["berat_kg"])
	}
	if decoded["jenis_ikan"] != "tuna" {
		t.Errorf("jenis_ikan = %v, want tuna", decoded["jenis_ikan"])
	}
	if decoded["fresh"] != true {
		t.Errorf("fresh = %v, want true", decoded["fresh"])
	}
}

func TestDecodePayloadProblems(t *testing.T) {
	// Missing field is reported but does not fail the whole packet.
	raw, _ := msgpack.Marshal(map[string]any{"lat": float32(1)})
	decoded, problems := DecodePayload(raw, sampleFields())
	if decoded == nil || len(problems) == 0 {
		t.Fatalf("expected partial decode with problems, got %v / %v", decoded, problems)
	}

	// Oversized string is kept but flagged.
	raw, _ = msgpack.Marshal(map[string]any{"jenis_ikan": "ikan tongkol besar sekali"})
	decoded, problems = DecodePayload(raw, []SchemaField{{Name: "jenis_ikan", Type: "string_10"}})
	if decoded["jenis_ikan"] != "ikan tongkol besar sekali" || len(problems) == 0 {
		t.Errorf("oversized string should be kept and flagged: %v / %v", decoded, problems)
	}
}

func TestDecodePayloadPositionalFallback(t *testing.T) {
	raw, _ := msgpack.Marshal([]any{float32(-6.5), float32(106.25), uint16(80), "layur", false})
	decoded, problems := DecodePayload(raw, sampleFields())
	if len(problems) != 0 {
		t.Fatalf("unexpected problems: %v", problems)
	}
	if w, ok := decoded["berat_kg"].(int64); !ok || w != 80 {
		t.Errorf("positional berat_kg = %v, want 80", decoded["berat_kg"])
	}
}

func TestDecodePayloadGarbage(t *testing.T) {
	decoded, problems := DecodePayload([]byte{0xc1, 0x00, 0xff}, sampleFields())
	if decoded != nil || len(problems) == 0 {
		t.Errorf("garbage must fail with problems, got %v / %v", decoded, problems)
	}
}
