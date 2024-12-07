// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sdktest // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/testing/sdktest"

import (
	"fmt"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	sdk "go.opentelemetry.io/otel/sdk/metric/metricdata"
	"gopkg.in/yaml.v3"
)

// Spec is the partial metric specification. To be used with [Compare]
type Spec = map[string]Metric

type Type string

const (
	TypeSum   Type = "sum"
	TypeGauge Type = "gauge"
)

type Metric struct {
	Type
	Name string

	Numbers     []Number
	Monotonic   bool
	Temporality sdk.Temporality
}

type Number struct {
	Int   *int64
	Float *float64
	Attr  attributes
}

// Unmarshal specification in [Format] into the given [Spec].
func Unmarshal(data Format, into *Spec) error {
	var doc map[string]yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return err
	}

	if *into == nil {
		*into = make(map[string]Metric, len(doc))
	}
	md := *into

	for key, node := range doc {
		args := strings.Fields(key)
		if len(args) < 2 {
			return fmt.Errorf("key must of form '<type> <name>', but got %q", key)
		}

		m := Metric{Name: args[1]}
		switch args[0] {
		case "counter":
			m.Type = TypeSum
			m.Monotonic = true
		case "updown":
			m.Type = TypeSum
			m.Monotonic = false
		case "gauge":
			m.Type = TypeGauge
		default:
			return fmt.Errorf("no such instrument type: %q", args[0])
		}

		m.Temporality = sdk.CumulativeTemporality
		for _, arg := range args[2:] {
			switch arg {
			case "delta":
				m.Temporality = sdk.DeltaTemporality
			case "cumulative":
				m.Temporality = sdk.CumulativeTemporality
			}
		}

		var into any
		switch m.Type {
		case TypeGauge, TypeSum:
			into = &m.Numbers
		default:
			panic("unreachable")
		}

		if err := node.Decode(into); err != nil {
			return err
		}

		md[m.Name] = m
	}

	return nil
}

type attributes map[string]string

func (attr attributes) Into() attribute.Set {
	kvs := make([]attribute.KeyValue, 0, len(attr))
	for k, v := range attr {
		kvs = append(kvs, attribute.String(k, v))
	}
	return attribute.NewSet(kvs...)
}

// Format defines the yaml-based format to be used with [Unmarshal] for specifying [Spec].
//
// It looks as follows:
//
//	<instrument> <name> [ delta|cumulative ]:
//	- int: <int64> | float: <float64>
//	  attr:
//	    [string]: <string>
//
// The supported instruments are:
//   - counter: [TypeSum], monotonic
//   - updown: [TypeSum], non-monotonic
//   - gauge: [TypeGauge]
//
// Temporality is optional and defaults to [sdk.CumulativeTemporality]
type Format = []byte
