package observability

import (
	"context"
	"io"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

const TracerName = "github.com/Defyland/pixrail-go-payment-switch"

func ConfigureTracing(serviceName string, writer io.Writer) func(context.Context) error {
	exporter, err := stdouttrace.New(stdouttrace.WithWriter(writer))
	if err != nil {
		return func(context.Context) error { return err }
	}
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(sdktrace.NewSimpleSpanProcessor(exporter)),
		sdktrace.WithResource(resource.NewSchemaless(attribute.String("service.name", serviceName))),
	)
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	return provider.Shutdown
}
