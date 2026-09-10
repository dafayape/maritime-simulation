package domain

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/vmihailenco/msgpack/v5"
)

// SchemaField is one attribute of the dynamic payload structure that the
// dashboard injects into every mobile node (SRS §3.2). Supported types are
// fixed-width primitives plus bounded strings ("string_10" = max 10 bytes),
// mirroring what a real ESP32 firmware would pack.
type SchemaField struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// DynamicSchema is the stored form of a schema (dynamic_schemas row).
type DynamicSchema struct {
	ID        int64         `json:"id"`
	SessionID string        `json:"session_id"`
	Fields    []SchemaField `json:"fields"`
	CreatedAt time.Time     `json:"created_at"`
}

const (
	maxSchemaFields    = 32
	maxStringFieldSize = 242 // never larger than the biggest SF payload
)

var (
	fieldNameRe  = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]{0,29}$`)
	stringTypeRe = regexp.MustCompile(`^string_([1-9][0-9]{0,2})$`)
)

// fixedTypeSizes lists the wire size in bytes of each primitive type.
var fixedTypeSizes = map[string]int{
	"float32": 4, "float64": 8,
	"int8": 1, "int16": 2, "int32": 4, "int64": 8,
	"uint8": 1, "uint16": 2, "uint32": 4, "uint64": 8,
	"bool": 1,
}

// FieldByteSize returns the nominal data size of a schema type, or an error
// for unknown types.
func FieldByteSize(fieldType string) (int, error) {
	if size, ok := fixedTypeSizes[fieldType]; ok {
		return size, nil
	}
	if m := stringTypeRe.FindStringSubmatch(fieldType); m != nil {
		n, err := strconv.Atoi(m[1])
		if err != nil || n < 1 || n > maxStringFieldSize {
			return 0, fmt.Errorf("%w: string field size must be 1..%d", ErrValidation, maxStringFieldSize)
		}
		return n, nil
	}
	return 0, fmt.Errorf("%w: unknown field type %q", ErrValidation, fieldType)
}

// ValidateSchemaFields enforces the structural rules of a dynamic schema.
func ValidateSchemaFields(fields []SchemaField) error {
	if len(fields) == 0 {
		return fmt.Errorf("%w: schema must define at least one field", ErrValidation)
	}
	if len(fields) > maxSchemaFields {
		return fmt.Errorf("%w: schema exceeds %d fields", ErrValidation, maxSchemaFields)
	}
	seen := make(map[string]bool, len(fields))
	for _, f := range fields {
		if !fieldNameRe.MatchString(f.Name) {
			return fmt.Errorf("%w: invalid field name %q (letters, digits, underscore, max 30 chars)", ErrValidation, f.Name)
		}
		if seen[f.Name] {
			return fmt.Errorf("%w: duplicate field name %q", ErrValidation, f.Name)
		}
		seen[f.Name] = true
		if _, err := FieldByteSize(f.Type); err != nil {
			return err
		}
	}
	return nil
}

// EstimatePackedBytes computes the worst-case MessagePack size of a payload
// following this schema (map header + per-field key string + value). The
// dashboard uses it to warn when a schema cannot fit the active SF window.
func EstimatePackedBytes(fields []SchemaField) int {
	total := 3 // map header (covers >15 fields; 1 byte for small maps)
	for _, f := range fields {
		total += 1 + len(f.Name) // fixstr key header + bytes
		size, err := FieldByteSize(f.Type)
		if err != nil {
			continue
		}
		if strings.HasPrefix(f.Type, "string_") {
			total += 2 + size // str8 header + max bytes
		} else {
			total += 1 + size // type marker + payload
		}
	}
	return total
}

// DecodePayload unpacks a raw MessagePack byte slice against the schema and
// coerces every field to a JSON-friendly value. It is deliberately lenient:
// a packet that survived up to five radio hops must not vanish at the Edge
// because of a single malformed field — problems are reported alongside the
// data instead (README: "lenient edge ingest").
func DecodePayload(raw []byte, fields []SchemaField) (map[string]any, []string) {
	var problems []string

	values := map[string]any{}
	if err := msgpack.Unmarshal(raw, &values); err != nil {
		// Fallback: positional array packing in schema field order.
		var arr []any
		if arrErr := msgpack.Unmarshal(raw, &arr); arrErr != nil {
			return nil, []string{fmt.Sprintf("msgpack decode failed: %v", err)}
		}
		values = map[string]any{}
		for i, f := range fields {
			if i < len(arr) {
				values[f.Name] = arr[i]
			}
		}
	}

	decoded := make(map[string]any, len(values))
	known := make(map[string]bool, len(fields))
	for _, f := range fields {
		known[f.Name] = true
		v, ok := values[f.Name]
		if !ok {
			problems = append(problems, fmt.Sprintf("missing field %q", f.Name))
			decoded[f.Name] = nil
			continue
		}
		coerced, err := coerceValue(v, f.Type)
		if err != nil {
			problems = append(problems, fmt.Sprintf("field %q: %v", f.Name, err))
			decoded[f.Name] = v
			continue
		}
		decoded[f.Name] = coerced
	}

	// Keep unknown extra keys — useful when the mobile app ships ahead of
	// the dashboard schema.
	for k, v := range values {
		if !known[k] {
			decoded[k] = v
		}
	}
	return decoded, problems
}

func coerceValue(v any, fieldType string) (any, error) {
	if m := stringTypeRe.FindStringSubmatch(fieldType); m != nil {
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("expected string, got %T", v)
		}
		maxLen, _ := strconv.Atoi(m[1])
		if len(s) > maxLen {
			return s, fmt.Errorf("string length %d exceeds declared max %d", len(s), maxLen)
		}
		return s, nil
	}

	switch fieldType {
	case "bool":
		b, ok := v.(bool)
		if !ok {
			return nil, fmt.Errorf("expected bool, got %T", v)
		}
		return b, nil
	case "float32", "float64":
		f, ok := toFloat64(v)
		if !ok {
			return nil, fmt.Errorf("expected number, got %T", v)
		}
		return f, nil
	default: // signed and unsigned integers
		n, ok := toInt64(v)
		if !ok {
			return nil, fmt.Errorf("expected integer, got %T", v)
		}
		return n, nil
	}
}

func toFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int8:
		return float64(n), true
	case int16:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint8:
		return float64(n), true
	case uint16:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	default:
		return 0, false
	}
}

func toInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case int:
		return int64(n), true
	case int8:
		return int64(n), true
	case int16:
		return int64(n), true
	case int32:
		return int64(n), true
	case int64:
		return n, true
	case uint8:
		return int64(n), true
	case uint16:
		return int64(n), true
	case uint32:
		return int64(n), true
	case uint64:
		return int64(n), true
	case float64:
		if n == float64(int64(n)) {
			return int64(n), true
		}
		return 0, false
	case float32:
		if float64(n) == float64(int64(n)) {
			return int64(n), true
		}
		return 0, false
	default:
		return 0, false
	}
}
