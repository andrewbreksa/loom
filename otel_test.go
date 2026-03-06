package loom_test

import (
	"context"
	"testing"

	"github.com/andrewbreksa/loom"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestDispatchContextCreatesSpan(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	defer tp.Shutdown(context.Background())

	rt := loom.New().
		WithTelemetry(loom.TelemetryOptions{
			TracerProvider:      tp,
			InstrumentationName: "loom-test",
		}).
		Ref("x", 0).
		Action("inc", loom.NewAction(
			func(_ loom.StateView, _ map[string]any) bool { return true },
			func(s loom.StateView, _ map[string]any) []loom.Rebind {
				return []loom.Rebind{s.Rebind("x", loom.Int(s.Get("x"))+1)}
			},
		)).
		Build()

	if err := rt.DispatchContext(context.Background(), "inc", nil); err != nil {
		t.Fatalf("dispatch failed: %v", err)
	}

	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("want 1 span, got %d", len(spans))
	}
	span := spans[0]

	if span.Name() != "loom.dispatch" {
		t.Fatalf("span name: want loom.dispatch, got %q", span.Name())
	}
	if span.Status().Code != codes.Ok {
		t.Fatalf("span status: want Ok, got %v", span.Status().Code)
	}
	if got, ok := stringAttr(span.Attributes(), "loom.action"); !ok || got != "inc" {
		t.Fatalf("loom.action: want inc, got %q (exists=%v)", got, ok)
	}
	if got, ok := stringAttr(span.Attributes(), "loom.result"); !ok || got != "ok" {
		t.Fatalf("loom.result: want ok, got %q (exists=%v)", got, ok)
	}
}

func TestDispatchContextPermitDeniedSpanStatus(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	defer tp.Shutdown(context.Background())

	rt := loom.New().
		WithTelemetry(loom.TelemetryOptions{
			TracerProvider:      tp,
			InstrumentationName: "loom-test",
		}).
		Action("blocked", loom.NewAction(
			func(_ loom.StateView, _ map[string]any) bool { return false },
			func(_ loom.StateView, _ map[string]any) []loom.Rebind {
				return []loom.Rebind{{Key: "x", Value: 1}}
			},
		)).
		Build()

	err := rt.DispatchContext(context.Background(), "blocked", nil)
	if err == nil {
		t.Fatal("expected permit error")
	}
	if _, ok := err.(loom.PermitError); !ok {
		t.Fatalf("want PermitError, got %T", err)
	}

	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("want 1 span, got %d", len(spans))
	}
	span := spans[0]

	if span.Status().Code != codes.Error {
		t.Fatalf("span status: want Error, got %v", span.Status().Code)
	}
	if got, ok := stringAttr(span.Attributes(), "loom.result"); !ok || got != "not_permitted" {
		t.Fatalf("loom.result: want not_permitted, got %q (exists=%v)", got, ok)
	}
}

func stringAttr(attrs []attribute.KeyValue, key string) (string, bool) {
	for _, kv := range attrs {
		if string(kv.Key) == key {
			return kv.Value.AsString(), true
		}
	}
	return "", false
}
