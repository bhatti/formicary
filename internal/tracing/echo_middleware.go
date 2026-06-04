package tracing

import (
	"fmt"

	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// EchoMiddleware returns an Echo middleware that creates HTTP server spans.
// Tracer and propagator are resolved per-request from the global provider,
// so this can be installed before Init() is called.
func EchoMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()
			propagator := otel.GetTextMapPropagator()
			ctx := propagator.Extract(req.Context(), propagation.HeaderCarrier(req.Header))

			spanName := fmt.Sprintf("HTTP %s %s", req.Method, c.Path())
			tracer := otel.GetTracerProvider().Tracer("formicary.http")
			ctx, span := tracer.Start(ctx, spanName,
				trace.WithSpanKind(trace.SpanKindServer),
				trace.WithAttributes(
					semconv.HTTPRequestMethodKey.String(req.Method),
					semconv.URLPath(req.URL.Path),
				),
			)
			defer span.End()

			c.SetRequest(req.WithContext(ctx))

			err := next(c)

			status := c.Response().Status
			span.SetAttributes(attribute.Int("http.response.status_code", status))
			if status >= 500 {
				span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d", status))
			}
			if err != nil {
				span.RecordError(err)
			}
			return err
		}
	}
}
