package middleware

import (
	"context"
	"log/slog"
	"strings"
	"sync"
)

// MaxRequestAttrs bounds handler-provided attributes on a request completion log.
const MaxRequestAttrs = 16

var protectedRequestAttrKeys = map[string]struct{}{
	"api_key":               {},
	"api_key_hash":          {},
	"authorization":         {},
	"credentials":           {},
	"credentials_encrypted": {},
	"duration_ms":           {},
	"key_hash":              {},
	"method":                {},
	"password":              {},
	"provider_token":        {},
	"request_id":            {},
	"route":                 {},
	"secret":                {},
	"status":                {},
	"token":                 {},
}

type requestContextKey struct{}

type requestContext struct {
	mu    sync.Mutex
	attrs []slog.Attr
}

func withRequestContext(ctx context.Context) (context.Context, *requestContext) {
	state := &requestContext{
		attrs: make([]slog.Attr, 0, MaxRequestAttrs),
	}
	return context.WithValue(ctx, requestContextKey{}, state), state
}

// AddAttrs enriches the request completion log associated with ctx. Attributes
// added outside RequestLogger, beyond MaxRequestAttrs, or with protected keys
// are ignored.
func AddAttrs(ctx context.Context, attrs ...slog.Attr) {
	state, ok := ctx.Value(requestContextKey{}).(*requestContext)
	if !ok {
		return
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	for _, attr := range attrs {
		if len(state.attrs) == MaxRequestAttrs {
			return
		}
		if !isSafeRequestAttr(attr) {
			continue
		}
		state.attrs = append(state.attrs, attr)
	}
}

func isSafeRequestAttr(attr slog.Attr) bool {
	key := strings.NewReplacer("-", "_", ".", "_").Replace(strings.ToLower(attr.Key))
	if key == "" {
		return false
	}
	if _, protected := protectedRequestAttrKeys[key]; protected {
		return false
	}

	value := attr.Value.Resolve()
	switch value.Kind() {
	case slog.KindAny:
		return false
	case slog.KindGroup:
		for _, child := range value.Group() {
			if !isSafeRequestAttr(child) {
				return false
			}
		}
	}
	return true
}

func (state *requestContext) attributes() []slog.Attr {
	state.mu.Lock()
	defer state.mu.Unlock()

	return append([]slog.Attr(nil), state.attrs...)
}
