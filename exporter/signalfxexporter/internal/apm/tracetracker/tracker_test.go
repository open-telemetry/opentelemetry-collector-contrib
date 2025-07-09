// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
// Originally copied from https://github.com/signalfx/signalfx-agent/blob/fbc24b0fdd3884bd0bbfbd69fe3c83f49d4c0b77/pkg/apm/tracetracker/tracker_test.go

package tracetracker

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/otel/semconv/v1.26.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/apm/correlations"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/apm/log"
)

func setTime(a *ActiveServiceTracker, t time.Time) {
	a.timeNow = func() time.Time { return t }
}

func advanceTime(a *ActiveServiceTracker, minutes int64) {
	newNow := a.timeNow().Add(time.Duration(minutes) * time.Minute)
	a.timeNow = func() time.Time { return newNow }
}

// newResourceWithAttrs creates a new resource with the given attributes.
func newResourceWithAttrs(maps ...map[string]string) pcommon.Resource {
	res := pcommon.NewResource()
	for _, m := range maps {
		for k, v := range m {
			res.Attributes().PutStr(k, v)
		}
	}
	return res
}

func TestExpiration(t *testing.T) {
	correlationClient := &correlationTestClient{}

	hostIDDims := map[string]string{"host": "test", "AWSUniqueId": "randomAWSUniqueId"}
	a := New(log.Nil, 5*time.Minute, correlationClient, hostIDDims, DefaultDimsToSyncSource)
	setTime(a, time.Unix(100, 0))

	fakeTraces := ptrace.NewTraces()
	newResourceWithAttrs(hostIDDims, map[string]string{string(conventions.ServiceNameKey): "one", "environment": "environment1"}).
		CopyTo(fakeTraces.ResourceSpans().AppendEmpty().Resource())
	newResourceWithAttrs(hostIDDims, map[string]string{string(conventions.ServiceNameKey): "two", "environment": "environment2"}).
		CopyTo(fakeTraces.ResourceSpans().AppendEmpty().Resource())
	newResourceWithAttrs(hostIDDims, map[string]string{string(conventions.ServiceNameKey): "three", "environment": "environment3"}).
		CopyTo(fakeTraces.ResourceSpans().AppendEmpty().Resource())
	a.ProcessTraces(context.Background(), fakeTraces)

	assert.Equal(t, int64(3), a.hostServiceCache.ActiveCount, "activeServiceCount is not properly tracked")
	assert.Equal(t, int64(3), a.hostEnvironmentCache.ActiveCount, "activeEnvironmentCount is not properly tracked")

	advanceTime(a, 4)

	fakeTraces = ptrace.NewTraces()
	newResourceWithAttrs(hostIDDims, map[string]string{string(conventions.ServiceNameKey): "two", "environment": "environment2"}).
		CopyTo(fakeTraces.ResourceSpans().AppendEmpty().Resource())
	a.ProcessTraces(context.Background(), fakeTraces)

	advanceTime(a, 2)
	a.Purge()

	assert.Equal(t, int64(1), a.hostServiceCache.ActiveCount, "activeServiceCount is not properly tracked")
	assert.Equal(t, int64(1), a.hostEnvironmentCache.ActiveCount, "activeEnvironmentCount is not properly tracked")
	assert.Equal(t, int64(2), a.hostServiceCache.PurgedCount, "purgedServiceCount is not properly tracked")
	assert.Equal(t, int64(2), a.hostEnvironmentCache.PurgedCount, "activeEnvironmentCount is not properly tracked")
}

type correlationTestClient struct {
	sync.Mutex
	cors             []*correlations.Correlation
	getPayload       map[string]map[string][]string
	getCallback      func()
	getCounter       int64
	deleteCounter    int64
	correlateCounter int64
}

func (c *correlationTestClient) Start()    { /*no-op*/ }
func (c *correlationTestClient) Shutdown() { /*no-op*/ }
func (c *correlationTestClient) Get(_ string, dimValue string, cb correlations.SuccessfulGetCB) {
	atomic.AddInt64(&c.getCounter, 1)
	go func() {
		cb(c.getPayload[dimValue])
		if c.getCallback != nil {
			c.getCallback()
		}
	}()
}

func (c *correlationTestClient) Correlate(cl *correlations.Correlation, cb correlations.CorrelateCB) {
	c.Lock()
	defer c.Unlock()
	c.cors = append(c.cors, cl)
	cb(cl, nil)
	atomic.AddInt64(&c.correlateCounter, 1)
}

func (c *correlationTestClient) Delete(cl *correlations.Correlation, cb correlations.SuccessfulDeleteCB) {
	c.Lock()
	defer c.Unlock()
	c.cors = append(c.cors, cl)
	cb(cl)
	atomic.AddInt64(&c.deleteCounter, 1)
}

func (c *correlationTestClient) getCorrelations() []*correlations.Correlation {
	c.Lock()
	defer c.Unlock()
	return c.cors
}

var _ correlations.CorrelationClient = &correlationTestClient{}

func TestCorrelationEmptyEnvironment(t *testing.T) {
	var wg sync.WaitGroup
	correlationClient := &correlationTestClient{
		getPayload: map[string]map[string][]string{
			"testk8sPodUID": {
				"sf_services":     {"one"},
				"sf_environments": {"environment1"},
			},
			"testContainerID": {
				"sf_services":     {"one"},
				"sf_environments": {"environment1"},
			},
		},
		getCallback: func() {
			wg.Done()
		},
	}

	hostIDDims := map[string]string{"host": "test", "AWSUniqueId": "randomAWSUniqueId"}
	wg.Add(len(hostIDDims))
	containerLevelIDDims := map[string]string{"kubernetes_pod_uid": "testk8sPodUID", "container_id": "testContainerID"}
	a := New(log.Nil, 5*time.Minute, correlationClient, hostIDDims, DefaultDimsToSyncSource)
	wg.Wait() // wait for the initial fetch of hostIDDims to complete

	fakeTraces := ptrace.NewTraces()
	fakeResource := newResourceWithAttrs(hostIDDims, containerLevelIDDims)
	fakeResource.CopyTo(fakeTraces.ResourceSpans().AppendEmpty().Resource())
	fakeResource.CopyTo(fakeTraces.ResourceSpans().AppendEmpty().Resource())
	fakeResource.CopyTo(fakeTraces.ResourceSpans().AppendEmpty().Resource())
	a.ProcessTraces(context.Background(), fakeTraces)

	cors := correlationClient.getCorrelations()
	assert.Len(t, cors, 4, "expected 4 correlations to be made")
	for _, c := range cors {
		assert.Contains(t, []string{"container_id", "kubernetes_pod_uid", "host", "AWSUniqueId"}, c.DimName)
		assert.Contains(t, []string{"test", "randomAWSUniqueId", "testk8sPodUID", "testContainerID"}, c.DimValue)
		assert.Equal(t, correlations.Type("environment"), c.Type)
		assert.Equal(t, fallbackEnvironment, c.Value)
	}
}

func TestCorrelationUpdates(t *testing.T) {
	var wg sync.WaitGroup
	correlationClient := &correlationTestClient{
		getPayload: map[string]map[string][]string{
			"test": {
				"sf_services":     {"one"},
				"sf_environments": {"environment1"},
			},
		},
		getCallback: func() {
			wg.Done()
		},
	}

	hostIDDims := map[string]string{"host": "test", "AWSUniqueId": "randomAWSUniqueId"}
	wg.Add(len(hostIDDims))
	containerLevelIDDims := map[string]string{"kubernetes_pod_uid": "testk8sPodUID", "container_id": "testContainerID"}
	a := New(log.Nil, 5*time.Minute, correlationClient, hostIDDims, DefaultDimsToSyncSource)
	wg.Wait()
	assert.Equal(t, int64(1), a.hostServiceCache.ActiveCount, "activeServiceCount is not properly tracked")
	assert.Equal(t, int64(1), a.hostEnvironmentCache.ActiveCount, "activeEnvironmentCount is not properly tracked")
	advanceTime(a, 6)
	a.Purge()
	assert.Equal(t, int64(0), a.hostServiceCache.ActiveCount, "activeServiceCount is not properly tracked")
	assert.Equal(t, int64(0), a.hostEnvironmentCache.ActiveCount, "activeEnvironmentCount is not properly tracked")
	assert.Equal(t, int64(1), a.hostServiceCache.PurgedCount, "hostServiceCache purged count is not properly tracked")
	assert.Equal(t, int64(1), a.hostEnvironmentCache.PurgedCount, "hostEnvironmentCache purged count is not properly tracked")

	setTime(a, time.Unix(100, 0))

	fakeTraces := ptrace.NewTraces()
	newResourceWithAttrs(containerLevelIDDims, map[string]string{string(conventions.ServiceNameKey): "one", "environment": "environment1"}).
		CopyTo(fakeTraces.ResourceSpans().AppendEmpty().Resource())
	newResourceWithAttrs(containerLevelIDDims, map[string]string{string(conventions.ServiceNameKey): "two", "environment": "environment2"}).
		CopyTo(fakeTraces.ResourceSpans().AppendEmpty().Resource())
	newResourceWithAttrs(containerLevelIDDims, map[string]string{string(conventions.ServiceNameKey): "three", "environment": "environment3"}).
		CopyTo(fakeTraces.ResourceSpans().AppendEmpty().Resource())
	a.ProcessTraces(context.Background(), fakeTraces)

	assert.Equal(t, int64(3), a.hostServiceCache.ActiveCount, "activeServiceCount is not properly tracked")
	assert.Equal(t, int64(3), a.hostEnvironmentCache.ActiveCount, "activeEnvironmentCount is not properly tracked")

	numEnvironments := 3
	numServices := 3
	numHostIDDimCorrelations := len(hostIDDims)*(numEnvironments+numServices) + 4 /* 4 deletes for service & environment fetched at startup */
	numContainerLevelCorrelations := 2 * len(containerLevelIDDims)
	totalExpectedCorrelations := numHostIDDimCorrelations + numContainerLevelCorrelations
	assert.Len(t, correlationClient.getCorrelations(), totalExpectedCorrelations, "# of correlation requests do not match")
}
