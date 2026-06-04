package tracing

import (
	"context"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

func TestInitDisabled(t *testing.T) {
	shutdown, err := Init(context.Background(), &Config{Enabled: false})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown error: %v", err)
	}
	// Should be a noop tracer
	tracer := otel.GetTracerProvider().Tracer("test")
	_, span := tracer.Start(context.Background(), "noop")
	if span.SpanContext().IsValid() {
		t.Fatal("expected invalid span context from noop provider")
	}
	span.End()
}

func TestInitNilConfig(t *testing.T) {
	shutdown, err := Init(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown error: %v", err)
	}
}

func TestTracerHelper(t *testing.T) {
	// Just verify it doesn't panic
	tracer := Tracer("test")
	if tracer == nil {
		t.Fatal("expected non-nil tracer")
	}
}

func TestInitEnabled(t *testing.T) {
	// Use a non-routable endpoint so the exporter doesn't actually connect,
	// but the provider initializes successfully.
	cfg := &Config{
		Enabled:      true,
		Endpoint:     "http://192.0.2.1:4318",
		ServiceName:  "test-service",
		SampleRatio:  1.0,
		BatchTimeout: 100 * time.Millisecond,
	}
	shutdown, err := Init(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = shutdown(context.Background()) }()

	tracer := otel.GetTracerProvider().Tracer("test")
	_, span := tracer.Start(context.Background(), "real-span")
	if !span.SpanContext().IsValid() {
		t.Fatal("expected valid span context from real provider")
	}
	if span.SpanContext().TraceID() == (trace.TraceID{}) {
		t.Fatal("expected non-zero trace ID")
	}
	span.End()
}
