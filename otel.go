package loom

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const defaultInstrumentationName = "github.com/andrewbreksa/loom"

// TelemetryOptions configures OpenTelemetry integration for Loom runtimes.
//
// If providers are nil, Loom uses the global providers from the otel package.
type TelemetryOptions struct {
	TracerProvider      trace.TracerProvider
	MeterProvider       metric.MeterProvider
	InstrumentationName string
}

func (o TelemetryOptions) normalized() TelemetryOptions {
	if o.TracerProvider == nil {
		o.TracerProvider = otel.GetTracerProvider()
	}
	if o.MeterProvider == nil {
		o.MeterProvider = otel.GetMeterProvider()
	}
	if o.InstrumentationName == "" {
		o.InstrumentationName = defaultInstrumentationName
	}
	return o
}

type runtimeTelemetry struct {
	tracer trace.Tracer

	dispatchTotal    metric.Int64Counter
	dispatchDuration metric.Float64Histogram
	rebindCount      metric.Int64Histogram
	watchCallbacks   metric.Int64Histogram
}

func newRuntimeTelemetry(options TelemetryOptions) *runtimeTelemetry {
	opts := options.normalized()
	meter := opts.MeterProvider.Meter(opts.InstrumentationName)

	dispatchTotal, _ := meter.Int64Counter(
		"loom.dispatch.total",
		metric.WithDescription("Total number of Loom dispatch calls."),
	)
	dispatchDuration, _ := meter.Float64Histogram(
		"loom.dispatch.duration.seconds",
		metric.WithDescription("Dispatch duration in seconds."),
	)
	rebindCount, _ := meter.Int64Histogram(
		"loom.dispatch.rebinds",
		metric.WithDescription("Number of rebinds emitted by a dispatch."),
	)
	watchCallbacks, _ := meter.Int64Histogram(
		"loom.dispatch.watch_callbacks",
		metric.WithDescription("Number of watch callbacks executed by a dispatch."),
	)

	return &runtimeTelemetry{
		tracer:           opts.TracerProvider.Tracer(opts.InstrumentationName),
		dispatchTotal:    dispatchTotal,
		dispatchDuration: dispatchDuration,
		rebindCount:      rebindCount,
		watchCallbacks:   watchCallbacks,
	}
}

func (t *runtimeTelemetry) startDispatch(ctx context.Context, action string, args map[string]any) (context.Context, trace.Span, time.Time) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, span := t.tracer.Start(ctx, "loom.dispatch", trace.WithAttributes(
		attribute.String("loom.action", action),
		attribute.Int("loom.args_count", len(args)),
	))
	return ctx, span, time.Now()
}

func (t *runtimeTelemetry) endDispatch(
	ctx context.Context,
	span trace.Span,
	action string,
	result string,
	rebindCount int,
	watchCallbacks int,
	duration time.Duration,
	err error,
) {
	attrs := []attribute.KeyValue{
		attribute.String("loom.action", action),
		attribute.String("loom.result", result),
	}

	if t.dispatchTotal != nil {
		t.dispatchTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
	}
	if t.dispatchDuration != nil {
		t.dispatchDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
	}
	if t.rebindCount != nil {
		t.rebindCount.Record(ctx, int64(rebindCount), metric.WithAttributes(attrs...))
	}
	if t.watchCallbacks != nil {
		t.watchCallbacks.Record(ctx, int64(watchCallbacks), metric.WithAttributes(attrs...))
	}

	span.SetAttributes(
		attribute.String("loom.result", result),
		attribute.Int("loom.rebind_count", rebindCount),
		attribute.Int("loom.watch_callback_count", watchCallbacks),
	)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "ok")
	}
	span.End()
}
