// Copyright 2020 OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build e2e
// +build e2e

package k8sattributesprocessor

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/multierr"
)

const (
	equal = iota
	regex
	exist
)

type expectedValue struct {
	mode  int
	value string
}

func newExpectedValue(mode int, value string) *expectedValue {
	return &expectedValue{
		mode:  mode,
		value: value,
	}
}

func TestTraceE2E(t *testing.T) {
	tcs := []struct {
		name    string
		service string
		attrs   map[string]*expectedValue
	}{
		{
			name:    "job",
			service: "test-job",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":                newExpectedValue(regex, "test-telemetrygen-job-[a-z0-9]*"),
				"k8s.pod.ip":                  newExpectedValue(exist, ""),
				"k8s.pod.uid":                 newExpectedValue(exist, ""),
				"k8s.pod.start_time":          newExpectedValue(exist, ""),
				"k8s.node.name":               newExpectedValue(exist, ""),
				"k8s.namespace.name":          newExpectedValue(equal, "default"),
				"k8s.job.name":                newExpectedValue(equal, "test-telemetrygen-job"),
				"k8s.job.uid":                 newExpectedValue(exist, ""),
				"k8s.annotations.workload":    newExpectedValue(equal, "job"),
				"k8s.labels.app":              newExpectedValue(equal, "test-telemetrygen"),
				"k8s.container.name":          newExpectedValue(equal, "telemetrygen"),
				"k8s.container.restart_count": newExpectedValue(equal, "0"),
				"container.image.name":        newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.tag":         newExpectedValue(equal, "latest"),
				"container.id":                newExpectedValue(exist, ""),
			},
		},
		{
			name:    "statefulset",
			service: "test-statefulset",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":                newExpectedValue(equal, "test-telemetrygen-statefulset-0"),
				"k8s.pod.ip":                  newExpectedValue(exist, ""),
				"k8s.pod.uid":                 newExpectedValue(exist, ""),
				"k8s.pod.start_time":          newExpectedValue(exist, ""),
				"k8s.node.name":               newExpectedValue(exist, ""),
				"k8s.namespace.name":          newExpectedValue(equal, "default"),
				"k8s.statefulset.name":        newExpectedValue(equal, "test-telemetrygen-statefulset"),
				"k8s.statefulset.uid":         newExpectedValue(exist, ""),
				"k8s.annotations.workload":    newExpectedValue(equal, "statefulset"),
				"k8s.labels.app":              newExpectedValue(equal, "test-telemetrygen-statefulset"),
				"k8s.container.name":          newExpectedValue(equal, "telemetrygen"),
				"k8s.container.restart_count": newExpectedValue(equal, "0"),
				"container.image.name":        newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.tag":         newExpectedValue(equal, "latest"),
				"container.id":                newExpectedValue(exist, ""),
			},
		},
		{
			name:    "deployment",
			service: "test-deployment",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":                newExpectedValue(regex, "test-telemetrygen-deployment-[a-z0-9]*-[a-z0-9]*"),
				"k8s.pod.ip":                  newExpectedValue(exist, ""),
				"k8s.pod.uid":                 newExpectedValue(exist, ""),
				"k8s.pod.start_time":          newExpectedValue(exist, ""),
				"k8s.node.name":               newExpectedValue(exist, ""),
				"k8s.namespace.name":          newExpectedValue(equal, "default"),
				"k8s.deployment.name":         newExpectedValue(equal, "test-telemetrygen-deployment"),
				"k8s.replicaset.name":         newExpectedValue(regex, "test-telemetrygen-deployment-[a-z0-9]*"),
				"k8s.replicaset.uid":          newExpectedValue(exist, ""),
				"k8s.annotations.workload":    newExpectedValue(equal, "deployment"),
				"k8s.labels.app":              newExpectedValue(equal, "test-telemetrygen-deployment"),
				"k8s.container.name":          newExpectedValue(equal, "telemetrygen"),
				"k8s.container.restart_count": newExpectedValue(equal, "0"),
				"container.image.name":        newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.tag":         newExpectedValue(equal, "latest"),
				"container.id":                newExpectedValue(exist, ""),
			},
		},
		{
			name:    "daemonset",
			service: "test-daemonset",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":                newExpectedValue(regex, "test-telemetrygen-daemonset-[a-z0-9]*"),
				"k8s.pod.ip":                  newExpectedValue(exist, ""),
				"k8s.pod.uid":                 newExpectedValue(exist, ""),
				"k8s.pod.start_time":          newExpectedValue(exist, ""),
				"k8s.node.name":               newExpectedValue(exist, ""),
				"k8s.namespace.name":          newExpectedValue(equal, "default"),
				"k8s.daemonset.name":          newExpectedValue(equal, "test-telemetrygen-daemonset"),
				"k8s.daemonset.uid":           newExpectedValue(exist, ""),
				"k8s.annotations.workload":    newExpectedValue(equal, "daemonset"),
				"k8s.labels.app":              newExpectedValue(equal, "test-telemetrygen-daemonset"),
				"k8s.container.name":          newExpectedValue(equal, "telemetrygen"),
				"k8s.container.restart_count": newExpectedValue(equal, "0"),
				"container.image.name":        newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.tag":         newExpectedValue(equal, "latest"),
				"container.id":                newExpectedValue(exist, ""),
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			scanSpanForAttributes(t, tc.service, tc.attrs)
		})
	}
}

func scanSpanForAttributes(t *testing.T, expectedService string, kvs map[string]*expectedValue) {

	f, _ := os.Open("./testdata/trace.json")
	defer f.Close()

	scanner := bufio.NewScanner(f)
	const maxCapacity int = 8388608
	buf := make([]byte, maxCapacity)
	scanner.Buffer(buf, maxCapacity)

	unmarshaler := ptrace.JSONUnmarshaler{}
	for scanner.Scan() {
		traces, serr := unmarshaler.UnmarshalTraces(scanner.Bytes())
		if serr != nil {
			continue
		}

		len := traces.ResourceSpans().Len()
		for i := 0; i < len; i++ {
			resourceSpans := traces.ResourceSpans().At(i)

			service, exist := resourceSpans.Resource().Attributes().Get("service.name")
			assert.Equal(t, true, exist, "span do not has 'service.name' attribute in resource")

			if service.AsString() != expectedService {
				continue
			}

			assert.NoError(t, resourceHasAttributes(resourceSpans.Resource(), kvs))
		}
	}

}

func resourceHasAttributes(resource pcommon.Resource, kvs map[string]*expectedValue) error {
	foundAttrs := make(map[string]bool)
	for k := range kvs {
		foundAttrs[k] = false
	}

	resource.Attributes().Range(
		func(k string, v pcommon.Value) bool {
			if val, ok := kvs[k]; ok {
				switch val.mode {
				case equal:
					if val.value == v.AsString() {
						foundAttrs[k] = true
					}
				case regex:
					matched, _ := regexp.MatchString(val.value, v.AsString())
					if matched {
						foundAttrs[k] = true
					}
				case exist:
					foundAttrs[k] = true
				}

			}
			return true
		},
	)

	var err error
	for k, v := range foundAttrs {
		if !v {
			err = multierr.Append(err, fmt.Errorf("%v attribute not found", k))
		}
	}
	return err
}
