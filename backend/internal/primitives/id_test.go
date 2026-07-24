package primitives

import (
	"regexp"
	"strings"
	"testing"
)

// TestContract_NewIDShape verifies each constructor produces a prefixed opaque
// string over [a-z0-9], as the API contract's Conventions section requires.
func TestContract_NewIDShape(t *testing.T) {
	cases := []struct {
		prefix string
		gen    func() string
	}{
		{"proj_", NewProjectID},
		{"ep_", NewEndpointID},
		{"prov_", NewProviderID},
		{"req_", NewRequestID},
		{"key_", NewAPIKeyID},
		{"snap_", NewSnapshotID},
	}
	body := regexp.MustCompile(`^[a-z0-9]{12}$`)
	for _, c := range cases {
		id := c.gen()
		if !strings.HasPrefix(id, c.prefix) {
			t.Errorf("id %q missing prefix %q", id, c.prefix)
		}
		if got := strings.TrimPrefix(id, c.prefix); !body.MatchString(got) {
			t.Errorf("id %q body = %q, want 12 base36 chars", id, got)
		}
	}
}

// TestContract_NewIDUnique guards against an obviously broken generator.
func TestContract_NewIDUnique(t *testing.T) {
	seen := make(map[string]struct{}, 1000)
	for range 1000 {
		id := NewRequestID()
		if _, dup := seen[id]; dup {
			t.Fatalf("duplicate ID generated: %q", id)
		}
		seen[id] = struct{}{}
	}
}
