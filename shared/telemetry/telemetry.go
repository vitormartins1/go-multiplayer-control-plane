// Package telemetry padroniza o setup de distributed tracing (OpenTelemetry)
// para todos os serviços. Ver ADR-0014 — Grafana Tempo via OTLP, propagação
// W3C trace context e sampling ParentBased. Cada serviço chama SetupTracing
// no wire-up e fecha o shutdown no graceful shutdown.
package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Config descreve o tracing de um serviço.
type Config struct {
	ServiceName string // nome do serviço (vira o atributo service.name)

	// OTLPEndpoint é o coletor OTLP gRPC no formato host:port (ex. "tempo:4317").
	// Vazio desliga o tracing — o tracer global continua no-op, útil em dev local
	// sem o Tempo no ar e em testes.
	OTLPEndpoint string

	// SampleRatio é a fração de traces-raiz amostrados (0..1). Aplicado via
	// ParentBased: se há trace pai, a decisão dele é respeitada; só os roots
	// passam por este ratio. 1.0 = amostra tudo (default local).
	SampleRatio float64
}

// SetupTracing configura o TracerProvider global, o exporter OTLP e o
// propagator W3C, e devolve uma função de shutdown que drena os spans
// pendentes. Quando OTLPEndpoint é vazio, devolve um shutdown no-op e não
// instala nada (tracing desligado).
func SetupTracing(ctx context.Context, cfg Config) (func(context.Context) error, error) {
	if cfg.OTLPEndpoint == "" {
		return func(context.Context) error { return nil }, nil
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(semconv.ServiceName(cfg.ServiceName)),
	)
	if err != nil {
		return nil, fmt.Errorf("telemetry: resource: %w", err)
	}

	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint),
		otlptracegrpc.WithInsecure(), // stack local sem TLS; ver ADR-0014
	)
	if err != nil {
		return nil, fmt.Errorf("telemetry: otlp exporter: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SampleRatio))),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, // W3C traceparent — ADR-0014
		propagation.Baggage{},
	))

	return tp.Shutdown, nil
}
