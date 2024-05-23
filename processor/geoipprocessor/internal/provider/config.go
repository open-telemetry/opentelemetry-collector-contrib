package provider

import "go.opentelemetry.io/collector/component"

// TODO: add common functionalities (e.g. set root path?)
type Config interface {
	component.Config
}
