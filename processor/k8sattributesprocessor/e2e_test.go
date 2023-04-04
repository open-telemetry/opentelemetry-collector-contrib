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
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
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
				"k8s.pod.name":                newExpectedValue(regex, "telemetrygen-traces-job-[a-z0-9]*"),
				"k8s.pod.ip":                  newExpectedValue(exist, ""),
				"k8s.pod.uid":                 newExpectedValue(exist, ""),
				"k8s.pod.start_time":          newExpectedValue(exist, ""),
				"k8s.node.name":               newExpectedValue(exist, ""),
				"k8s.namespace.name":          newExpectedValue(equal, "default"),
				"k8s.job.name":                newExpectedValue(equal, "telemetrygen-traces-job"),
				"k8s.job.uid":                 newExpectedValue(exist, ""),
				"k8s.annotations.workload":    newExpectedValue(equal, "job"),
				"k8s.labels.app":              newExpectedValue(equal, "telemetrygen-traces"),
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
				"k8s.pod.name":                newExpectedValue(equal, "telemetrygen-traces-statefulset-0"),
				"k8s.pod.ip":                  newExpectedValue(exist, ""),
				"k8s.pod.uid":                 newExpectedValue(exist, ""),
				"k8s.pod.start_time":          newExpectedValue(exist, ""),
				"k8s.node.name":               newExpectedValue(exist, ""),
				"k8s.namespace.name":          newExpectedValue(equal, "default"),
				"k8s.statefulset.name":        newExpectedValue(equal, "telemetrygen-traces-statefulset"),
				"k8s.statefulset.uid":         newExpectedValue(exist, ""),
				"k8s.annotations.workload":    newExpectedValue(equal, "statefulset"),
				"k8s.labels.app":              newExpectedValue(equal, "telemetrygen-traces-statefulset"),
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
				"k8s.pod.name":                newExpectedValue(regex, "telemetrygen-traces-deployment-[a-z0-9]*-[a-z0-9]*"),
				"k8s.pod.ip":                  newExpectedValue(exist, ""),
				"k8s.pod.uid":                 newExpectedValue(exist, ""),
				"k8s.pod.start_time":          newExpectedValue(exist, ""),
				"k8s.node.name":               newExpectedValue(exist, ""),
				"k8s.namespace.name":          newExpectedValue(equal, "default"),
				"k8s.deployment.name":         newExpectedValue(equal, "telemetrygen-traces-deployment"),
				"k8s.replicaset.name":         newExpectedValue(regex, "telemetrygen-traces-deployment-[a-z0-9]*"),
				"k8s.replicaset.uid":          newExpectedValue(exist, ""),
				"k8s.annotations.workload":    newExpectedValue(equal, "deployment"),
				"k8s.labels.app":              newExpectedValue(equal, "telemetrygen-traces-deployment"),
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
				"k8s.pod.name":                newExpectedValue(regex, "telemetrygen-traces-daemonset-[a-z0-9]*"),
				"k8s.pod.ip":                  newExpectedValue(exist, ""),
				"k8s.pod.uid":                 newExpectedValue(exist, ""),
				"k8s.pod.start_time":          newExpectedValue(exist, ""),
				"k8s.node.name":               newExpectedValue(exist, ""),
				"k8s.namespace.name":          newExpectedValue(equal, "default"),
				"k8s.daemonset.name":          newExpectedValue(equal, "telemetrygen-traces-daemonset"),
				"k8s.daemonset.uid":           newExpectedValue(exist, ""),
				"k8s.annotations.workload":    newExpectedValue(equal, "daemonset"),
				"k8s.labels.app":              newExpectedValue(equal, "telemetrygen-traces-daemonset"),
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
	var err error
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
			err = resourceHasAttributes(resourceSpans.Resource(), kvs)
		}
	}
	assert.NoError(t, err)

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

func TestMetricE2E(t *testing.T) {
	tcs := []struct {
		name    string
		service string
		attrs   map[string]*expectedValue
	}{
		{
			name:    "job",
			service: "test-job",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":                newExpectedValue(regex, "telemetrygen-metrics-job-[a-z0-9]*"),
				"k8s.pod.ip":                  newExpectedValue(exist, ""),
				"k8s.pod.uid":                 newExpectedValue(exist, ""),
				"k8s.pod.start_time":          newExpectedValue(exist, ""),
				"k8s.node.name":               newExpectedValue(exist, ""),
				"k8s.namespace.name":          newExpectedValue(equal, "default"),
				"k8s.job.name":                newExpectedValue(equal, "telemetrygen-metrics-job"),
				"k8s.job.uid":                 newExpectedValue(exist, ""),
				"k8s.annotations.workload":    newExpectedValue(equal, "job"),
				"k8s.labels.app":              newExpectedValue(equal, "telemetrygen-metrics"),
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
				"k8s.pod.name":                newExpectedValue(equal, "telemetrygen-metrics-statefulset-0"),
				"k8s.pod.ip":                  newExpectedValue(exist, ""),
				"k8s.pod.uid":                 newExpectedValue(exist, ""),
				"k8s.pod.start_time":          newExpectedValue(exist, ""),
				"k8s.node.name":               newExpectedValue(exist, ""),
				"k8s.namespace.name":          newExpectedValue(equal, "default"),
				"k8s.statefulset.name":        newExpectedValue(equal, "telemetrygen-metrics-statefulset"),
				"k8s.statefulset.uid":         newExpectedValue(exist, ""),
				"k8s.annotations.workload":    newExpectedValue(equal, "statefulset"),
				"k8s.labels.app":              newExpectedValue(equal, "telemetrygen-metrics-statefulset"),
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
				"k8s.pod.name":                newExpectedValue(regex, "telemetrygen-metrics-deployment-[a-z0-9]*-[a-z0-9]*"),
				"k8s.pod.ip":                  newExpectedValue(exist, ""),
				"k8s.pod.uid":                 newExpectedValue(exist, ""),
				"k8s.pod.start_time":          newExpectedValue(exist, ""),
				"k8s.node.name":               newExpectedValue(exist, ""),
				"k8s.namespace.name":          newExpectedValue(equal, "default"),
				"k8s.deployment.name":         newExpectedValue(equal, "telemetrygen-metrics-deployment"),
				"k8s.replicaset.name":         newExpectedValue(regex, "telemetrygen-metrics-deployment-[a-z0-9]*"),
				"k8s.replicaset.uid":          newExpectedValue(exist, ""),
				"k8s.annotations.workload":    newExpectedValue(equal, "deployment"),
				"k8s.labels.app":              newExpectedValue(equal, "telemetrygen-metrics-deployment"),
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
				"k8s.pod.name":                newExpectedValue(regex, "telemetrygen-metrics-daemonset-[a-z0-9]*"),
				"k8s.pod.ip":                  newExpectedValue(exist, ""),
				"k8s.pod.uid":                 newExpectedValue(exist, ""),
				"k8s.pod.start_time":          newExpectedValue(exist, ""),
				"k8s.node.name":               newExpectedValue(exist, ""),
				"k8s.namespace.name":          newExpectedValue(equal, "default"),
				"k8s.daemonset.name":          newExpectedValue(equal, "telemetrygen-metrics-daemonset"),
				"k8s.daemonset.uid":           newExpectedValue(exist, ""),
				"k8s.annotations.workload":    newExpectedValue(equal, "daemonset"),
				"k8s.labels.app":              newExpectedValue(equal, "telemetrygen-metrics-daemonset"),
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
			scanMetricForAttributes(t, tc.service, tc.attrs)
		})
	}
}

func scanMetricForAttributes(t *testing.T, expectedService string, kvs map[string]*expectedValue) {

	f, _ := os.Open("./testdata/metric.json")
	defer f.Close()

	scanner := bufio.NewScanner(f)
	const maxCapacity int = 8388608
	buf := make([]byte, maxCapacity)
	scanner.Buffer(buf, maxCapacity)

	unmarshaler := pmetric.JSONUnmarshaler{}
	var err error
	for scanner.Scan() {
		metrics, serr := unmarshaler.UnmarshalMetrics(scanner.Bytes())
		if serr != nil {
			continue
		}

		len := metrics.ResourceMetrics().Len()
		for i := 0; i < len; i++ {
			resourceMetrics := metrics.ResourceMetrics().At(i)

			service, exist := resourceMetrics.Resource().Attributes().Get("service.name")
			assert.Equal(t, true, exist, "metric do not has 'service.name' attribute in resource")

			if service.AsString() != expectedService {
				continue
			}
			err = resourceHasAttributes(resourceMetrics.Resource(), kvs)
		}
	}
	assert.NoError(t, err)

}

func TestLogE2E(t *testing.T) {
	tcs := []struct {
		name    string
		service string
		attrs   map[string]*expectedValue
	}{
		{
			name:    "job",
			service: "test-job",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":                newExpectedValue(regex, "telemetrygen-logs-job-[a-z0-9]*"),
				"k8s.pod.ip":                  newExpectedValue(exist, ""),
				"k8s.pod.uid":                 newExpectedValue(exist, ""),
				"k8s.pod.start_time":          newExpectedValue(exist, ""),
				"k8s.node.name":               newExpectedValue(exist, ""),
				"k8s.namespace.name":          newExpectedValue(equal, "default"),
				"k8s.job.name":                newExpectedValue(equal, "telemetrygen-logs-job"),
				"k8s.job.uid":                 newExpectedValue(exist, ""),
				"k8s.annotations.workload":    newExpectedValue(equal, "job"),
				"k8s.labels.app":              newExpectedValue(equal, "telemetrygen-logs"),
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
				"k8s.pod.name":                newExpectedValue(equal, "telemetrygen-logs-statefulset-0"),
				"k8s.pod.ip":                  newExpectedValue(exist, ""),
				"k8s.pod.uid":                 newExpectedValue(exist, ""),
				"k8s.pod.start_time":          newExpectedValue(exist, ""),
				"k8s.node.name":               newExpectedValue(exist, ""),
				"k8s.namespace.name":          newExpectedValue(equal, "default"),
				"k8s.statefulset.name":        newExpectedValue(equal, "telemetrygen-logs-statefulset"),
				"k8s.statefulset.uid":         newExpectedValue(exist, ""),
				"k8s.annotations.workload":    newExpectedValue(equal, "statefulset"),
				"k8s.labels.app":              newExpectedValue(equal, "telemetrygen-logs-statefulset"),
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
				"k8s.pod.name":                newExpectedValue(regex, "telemetrygen-logs-deployment-[a-z0-9]*-[a-z0-9]*"),
				"k8s.pod.ip":                  newExpectedValue(exist, ""),
				"k8s.pod.uid":                 newExpectedValue(exist, ""),
				"k8s.pod.start_time":          newExpectedValue(exist, ""),
				"k8s.node.name":               newExpectedValue(exist, ""),
				"k8s.namespace.name":          newExpectedValue(equal, "default"),
				"k8s.deployment.name":         newExpectedValue(equal, "telemetrygen-logs-deployment"),
				"k8s.replicaset.name":         newExpectedValue(regex, "telemetrygen-logs-deployment-[a-z0-9]*"),
				"k8s.replicaset.uid":          newExpectedValue(exist, ""),
				"k8s.annotations.workload":    newExpectedValue(equal, "deployment"),
				"k8s.labels.app":              newExpectedValue(equal, "telemetrygen-logs-deployment"),
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
				"k8s.pod.name":                newExpectedValue(regex, "telemetrygen-logs-daemonset-[a-z0-9]*"),
				"k8s.pod.ip":                  newExpectedValue(exist, ""),
				"k8s.pod.uid":                 newExpectedValue(exist, ""),
				"k8s.pod.start_time":          newExpectedValue(exist, ""),
				"k8s.node.name":               newExpectedValue(exist, ""),
				"k8s.namespace.name":          newExpectedValue(equal, "default"),
				"k8s.daemonset.name":          newExpectedValue(equal, "telemetrygen-logs-daemonset"),
				"k8s.daemonset.uid":           newExpectedValue(exist, ""),
				"k8s.annotations.workload":    newExpectedValue(equal, "daemonset"),
				"k8s.labels.app":              newExpectedValue(equal, "telemetrygen-logs-daemonset"),
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
			scanLogForAttributes(t, tc.service, tc.attrs)
		})
	}
}

func scanLogForAttributes(t *testing.T, expectedService string, kvs map[string]*expectedValue) {

	f, _ := os.Open("./testdata/log.json")
	defer f.Close()

	scanner := bufio.NewScanner(f)
	const maxCapacity int = 8388608
	buf := make([]byte, maxCapacity)
	scanner.Buffer(buf, maxCapacity)

	unmarshaler := plog.JSONUnmarshaler{}
	var err error
	for scanner.Scan() {
		logs, serr := unmarshaler.UnmarshalLogs(scanner.Bytes())
		if serr != nil {
			continue
		}

		len := logs.ResourceLogs().Len()
		for i := 0; i < len; i++ {
			resourceLogs := logs.ResourceLogs().At(i)

			service, exist := resourceLogs.Resource().Attributes().Get("service.name")
			assert.Equal(t, true, exist, "log do not has 'service.name' attribute in resource")

			if service.AsString() != expectedService {
				continue
			}
			err = resourceHasAttributes(resourceLogs.Resource(), kvs)
		}
	}
	assert.NoError(t, err)

}
