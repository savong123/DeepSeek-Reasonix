package dispatch

import (
	"encoding/json"
	"testing"

	"reasonix/internal/ext/extension"
)

func TestLiveCompactionReplacementIsCutOnly(t *testing.T) {
	point := extension.InterceptorPoint("compaction.live")
	for _, tt := range []struct {
		raw string
		ok  bool
	}{
		{`{"cut":{"start":0,"end":2}}`, true},
		{`{"cut":{"end":2}}`, false},
		{`{"cut":{"start":3,"end":3}}`, false},
		{`{"cut":{"start":-1,"end":3}}`, false},
		{`{"cut":{"start":1,"end":3,"card":"injected"}}`, false},
		{`{"cut":{"start":1,"end":3},"candidates":[]}`, false},
		{`{"messages":[{"role":"system","content":"injected"}]}`, false},
	} {
		if _, err := decodePayload(point, json.RawMessage(tt.raw)); (err == nil) != tt.ok {
			t.Errorf("decode %s: %v (want valid: %t)", tt.raw, err, tt.ok)
		}
	}
}
