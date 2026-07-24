package primitives

import "crypto/rand"

const (
	prefixProject  = "proj_"
	prefixEndpoint = "ep_"
	prefixProvider = "prov_"
	prefixRequest  = "req_"
	prefixAPIKey   = "key_"
	prefixSnapshot = "snap_"
)

const idAlphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
const idBodyLen = 12

func NewProjectID() string  { return newID(prefixProject) }
func NewEndpointID() string { return newID(prefixEndpoint) }
func NewProviderID() string { return newID(prefixProvider) }
func NewRequestID() string  { return newID(prefixRequest) }
func NewAPIKeyID() string   { return newID(prefixAPIKey) }
func NewSnapshotID() string { return newID(prefixSnapshot) }

// newID returns a prefixed, random, opaque identifier, e.g. "ep_x9y8z7a1b2c3".
// It panics only if the system CSPRNG fails.
func newID(prefix string) string {
	buf := make([]byte, idBodyLen)
	if _, err := rand.Read(buf); err != nil {
		panic("primitives: reading random bytes for ID: " + err.Error())
	}
	for i, b := range buf {
		buf[i] = idAlphabet[int(b)%len(idAlphabet)]
	}
	return prefix + string(buf)
}
