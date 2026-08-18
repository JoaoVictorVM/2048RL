package metrics

import (
	"encoding/json"
	"testing"
)

func TestRecord_JSONRoundTrip(t *testing.T) {
	original := Record{Episode: 42, Score: 12345, MaxTile: 2048, Won: true, Moves: 987}

	encoded, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded Record
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if decoded != original {
		t.Errorf("round-trip alterou o registro: %+v", decoded)
	}
}

func TestRecord_JSONFieldNames(t *testing.T) {
	encoded, err := json.Marshal(Record{Episode: 1, Score: 2, MaxTile: 4, Won: false, Moves: 8})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var fields map[string]any
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	for _, name := range []string{"episode", "score", "max_tile", "won", "moves"} {
		if _, ok := fields[name]; !ok {
			t.Errorf("campo %q ausente no JSON: %s", name, encoded)
		}
	}
	if len(fields) != 5 {
		t.Errorf("esperados 5 campos, obtidos %d: %s", len(fields), encoded)
	}
}
