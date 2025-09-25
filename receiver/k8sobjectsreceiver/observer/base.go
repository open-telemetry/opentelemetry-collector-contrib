package observer

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	apiWatch "k8s.io/apimachinery/pkg/watch"
	"time"
)

type Mode string

const (
	PullMode  Mode = "pull"
	WatchMode Mode = "watch"

	DefaultMode = PullMode
)

type Config struct {
	Gvr                 schema.GroupVersionResource
	Namespaces          []string
	Interval            time.Duration
	LabelSelector       string
	FieldSelector       string
	ResourceVersion     string
	IncludeInitialState bool
	Exclude             map[apiWatch.EventType]bool
}
