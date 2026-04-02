package traceutil

import (
	"context"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

type amqpHeaderCarrier struct {
	headers amqp.Table
}

func (c amqpHeaderCarrier) Get(key string) string {
	if c.headers == nil {
		return ""
	}

	val, ok := c.headers[key]
	if !ok {
		return ""
	}

	switch v := val.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		return fmt.Sprint(v)
	}
}

func (c amqpHeaderCarrier) Set(key, value string) {
	if c.headers == nil {
		return
	}

	c.headers[key] = value
}

func (c amqpHeaderCarrier) Keys() []string {
	if c.headers == nil {
		return nil
	}

	keys := make([]string, 0, len(c.headers))
	for key := range c.headers {
		keys = append(keys, key)
	}

	return keys
}

func InjectMap(ctx context.Context) map[string]string {
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	if len(carrier) == 0 {
		return nil
	}

	result := make(map[string]string, len(carrier))
	for key, value := range carrier {
		result[key] = value
	}

	return result
}

func ExtractContext(ctx context.Context, carrier map[string]string) context.Context {
	if len(carrier) == 0 {
		return ctx
	}

	return otel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier(carrier))
}

func InjectAMQPHeaders(ctx context.Context, headers amqp.Table) amqp.Table {
	if headers == nil {
		headers = amqp.Table{}
	}

	otel.GetTextMapPropagator().Inject(ctx, amqpHeaderCarrier{headers: headers})
	return headers
}

func ExtractAMQPContext(ctx context.Context, headers amqp.Table) context.Context {
	if len(headers) == 0 {
		return ctx
	}

	return otel.GetTextMapPropagator().Extract(ctx, amqpHeaderCarrier{headers: headers})
}
