package tracing

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func TestInjectExtractRoundTrip(t *testing.T) {
	// Set up a real tracer provider for testing
	tp := sdktrace.NewTracerProvider()
	defer func() { _ = tp.Shutdown(context.Background()) }()
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Create a span to produce a valid trace context
	tracer := tp.Tracer("test")
	ctx, span := tracer.Start(context.Background(), "test-span")
	defer span.End()

	// Inject into headers
	headers := make(map[string]string)
	InjectContext(ctx, headers)

	if _, ok := headers["traceparent"]; !ok {
		t.Fatal("expected traceparent header to be injected")
	}

	// Extract from headers into a new context
	newCtx := ExtractContext(context.Background(), headers)
	sc := trace.SpanContextFromContext(newCtx)

	if !sc.IsValid() {
		t.Fatal("expected valid span context after extraction")
	}
	if sc.TraceID() != span.SpanContext().TraceID() {
		t.Errorf("trace IDs don't match: got %s, want %s",
			sc.TraceID(), span.SpanContext().TraceID())
	}
}

func TestInjectNilHeaders(t *testing.T) {
	// Should not panic
	InjectContext(context.Background(), nil)
}

func TestExtractNilHeaders(t *testing.T) {
	ctx := ExtractContext(context.Background(), nil)
	sc := trace.SpanContextFromContext(ctx)
	if sc.IsValid() {
		t.Fatal("expected invalid span context from nil headers")
	}
}

func TestMessageHeadersCarrierKeys(t *testing.T) {
	carrier := MessageHeadersCarrier(map[string]string{
		"traceparent": "00-abc-def-01",
		"tracestate":  "vendor=value",
	})
	keys := carrier.Keys()
	if len(keys) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(keys))
	}
}
