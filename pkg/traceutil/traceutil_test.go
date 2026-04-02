package traceutil

import (
	"context"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func TestAMQPContextRoundTrip(t *testing.T) {
	originalPropagator := otel.GetTextMapPropagator()
	originalProvider := otel.GetTracerProvider()
	otel.SetTextMapPropagator(propagation.TraceContext{})
	defer otel.SetTextMapPropagator(originalPropagator)

	provider := tracesdk.NewTracerProvider()
	otel.SetTracerProvider(provider)
	defer func() {
		_ = provider.Shutdown(context.Background())
		otel.SetTracerProvider(originalProvider)
	}()

	ctx, span := provider.Tracer("test").Start(context.Background(), "root")
	defer span.End()

	headers := InjectAMQPHeaders(ctx, nil)
	if len(headers) == 0 {
		t.Fatalf("expected trace headers to be injected")
	}

	extractedCtx := ExtractAMQPContext(context.Background(), headers)
	spanCtx := trace.SpanContextFromContext(extractedCtx)
	if !spanCtx.IsValid() {
		t.Fatalf("expected valid span context after extraction")
	}
	if got, want := spanCtx.TraceID(), span.SpanContext().TraceID(); got != want {
		t.Fatalf("unexpected trace id, got %s want %s", got, want)
	}
}

func TestMapContextRoundTrip(t *testing.T) {
	originalPropagator := otel.GetTextMapPropagator()
	originalProvider := otel.GetTracerProvider()
	otel.SetTextMapPropagator(propagation.TraceContext{})
	defer otel.SetTextMapPropagator(originalPropagator)

	provider := tracesdk.NewTracerProvider()
	otel.SetTracerProvider(provider)
	defer func() {
		_ = provider.Shutdown(context.Background())
		otel.SetTracerProvider(originalProvider)
	}()

	ctx, span := provider.Tracer("test").Start(context.Background(), "root")
	defer span.End()

	carrier := InjectMap(ctx)
	if len(carrier) == 0 {
		t.Fatalf("expected trace map to be injected")
	}

	extractedCtx := ExtractContext(context.Background(), carrier)
	spanCtx := trace.SpanContextFromContext(extractedCtx)
	if !spanCtx.IsValid() {
		t.Fatalf("expected valid span context after extraction")
	}
	if got, want := spanCtx.TraceID(), span.SpanContext().TraceID(); got != want {
		t.Fatalf("unexpected trace id, got %s want %s", got, want)
	}
}

func TestExtractAMQPContextWithEmptyHeaders(t *testing.T) {
	ctx := context.WithValue(context.Background(), struct{}{}, "keep")
	got := ExtractAMQPContext(ctx, amqp.Table{})
	if got != ctx {
		t.Fatalf("expected original context when headers are empty")
	}
}
