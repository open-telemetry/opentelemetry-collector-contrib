// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sinventory // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sinventory"

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
