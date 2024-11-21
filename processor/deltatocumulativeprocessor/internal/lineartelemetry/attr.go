package telemetry

import "go.opentelemetry.io/otel/attribute"

type Attributes []attribute.KeyValue

func (a *Attributes) Set(attr attribute.KeyValue) {
	*a = append(*a, attr)
}
