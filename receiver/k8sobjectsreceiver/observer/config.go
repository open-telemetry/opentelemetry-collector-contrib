package observer // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sobjectsreceiver/observer"

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type Mode string

const (
	PullMode  Mode = "pull"
	WatchMode Mode = "watch"

	DefaultMode = PullMode
)

type Config struct {
	Gvr             schema.GroupVersionResource
	Namespaces      []string
	LabelSelector   string
	FieldSelector   string
	ResourceVersion string
}
