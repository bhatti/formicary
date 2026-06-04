package tracing

import (
	"context"

	"go.opentelemetry.io/otel"
)

// MessageHeadersCarrier adapts a map[string]string to propagation.TextMapCarrier
// for injecting/extracting W3C TraceContext through queue message headers.
type MessageHeadersCarrier map[string]string

func (c MessageHeadersCarrier) Get(key string) string {
	return c[key]
}

func (c MessageHeadersCarrier) Set(key, value string) {
	c[key] = value
}

func (c MessageHeadersCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}

// InjectContext writes the span context from ctx into the provided headers map.
func InjectContext(ctx context.Context, headers map[string]string) {
	if headers == nil {
		return
	}
	otel.GetTextMapPropagator().Inject(ctx, MessageHeadersCarrier(headers))
}

// ExtractContext reads span context from headers and returns a new context with that parent.
func ExtractContext(ctx context.Context, headers map[string]string) context.Context {
	if headers == nil {
		return ctx
	}
	return otel.GetTextMapPropagator().Extract(ctx, MessageHeadersCarrier(headers))
}
