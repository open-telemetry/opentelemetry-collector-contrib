// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resourcedetectionprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor"

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

type resourceDetectionProcessor struct {
	provider           *internal.ResourceProvider
	resource           pcommon.Resource
	schemaURL          string
	override           bool
	httpClientSettings confighttp.ClientConfig
	refreshInterval    time.Duration
	telemetrySettings  component.TelemetrySettings

	stopCh  chan struct{}
	wg      sync.WaitGroup
	current atomic.Value
}

type resourceSnapshot struct {
	res       pcommon.Resource
	schemaURL string
}

// Start is invoked during service startup.
func (rdp *resourceDetectionProcessor) Start(ctx context.Context, host component.Host) error {
	client, _ := rdp.httpClientSettings.ToClient(ctx, host, rdp.telemetrySettings)
	ctx = internal.ContextWithClient(ctx, client)
	var err error
	rdp.resource, rdp.schemaURL, err = rdp.provider.Get(ctx, client)
	if err != nil {
		return err
	}

	rdp.current.Store(&resourceSnapshot{
		res:       cloneResource(rdp.resource),
		schemaURL: rdp.schemaURL,
	})

	if rdp.refreshInterval > 0 {
		rdp.stopCh = make(chan struct{})
		rdp.wg.Add(1)
		go rdp.refreshLoop(client)
	}
	return nil
}

// Shutdown is invoked during service shutdown.
func (rdp *resourceDetectionProcessor) Shutdown(_ context.Context) error {
	if rdp.stopCh != nil {
		close(rdp.stopCh)
		rdp.wg.Wait()
	}
	return nil
}

func (rdp *resourceDetectionProcessor) refreshLoop(client *http.Client) {
	defer rdp.wg.Done()
	ticker := time.NewTicker(rdp.refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			res, schemaURL, err := rdp.provider.Refresh(context.Background(), client)
			if err != nil {
				rdp.telemetrySettings.Logger.Warn("resource refresh failed", zap.Error(err))
				continue
			}

			rdp.resource = res
			rdp.schemaURL = schemaURL
			rdp.current.Store(&resourceSnapshot{
				res:       cloneResource(res),
				schemaURL: schemaURL,
			})
		case <-rdp.stopCh:
			return
		}
	}
}

func cloneResource(src pcommon.Resource) pcommon.Resource {
	dst := pcommon.NewResource()
	src.CopyTo(dst)
	return dst
}

// processTraces implements the ProcessTracesFunc type.
func (rdp *resourceDetectionProcessor) processTraces(_ context.Context, td ptrace.Traces) (ptrace.Traces, error) {
	snap, _ := rdp.current.Load().(*resourceSnapshot)
	rs := td.ResourceSpans()
	for i := 0; i < rs.Len(); i++ {
		rss := rs.At(i)
		if snap != nil {
			rss.SetSchemaUrl(internal.MergeSchemaURL(rss.SchemaUrl(), snap.schemaURL))
			res := rss.Resource()
			internal.MergeResource(res, snap.res, rdp.override)
		}
	}
	return td, nil
}

// processMetrics implements the ProcessMetricsFunc type.
func (rdp *resourceDetectionProcessor) processMetrics(_ context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	snap, _ := rdp.current.Load().(*resourceSnapshot)
	rm := md.ResourceMetrics()
	for i := 0; i < rm.Len(); i++ {
		rss := rm.At(i)
		if snap != nil {
			rss.SetSchemaUrl(internal.MergeSchemaURL(rss.SchemaUrl(), snap.schemaURL))
			res := rss.Resource()
			internal.MergeResource(res, snap.res, rdp.override)
		}
	}
	return md, nil
}

// processLogs implements the ProcessLogsFunc type.
func (rdp *resourceDetectionProcessor) processLogs(_ context.Context, ld plog.Logs) (plog.Logs, error) {
	snap, _ := rdp.current.Load().(*resourceSnapshot)
	rl := ld.ResourceLogs()
	for i := 0; i < rl.Len(); i++ {
		rss := rl.At(i)
		if snap != nil {
			rss.SetSchemaUrl(internal.MergeSchemaURL(rss.SchemaUrl(), snap.schemaURL))
			res := rss.Resource()
			internal.MergeResource(res, snap.res, rdp.override)
		}
	}
	return ld, nil
}

// processProfiles implements the ProcessProfilesFunc type.
func (rdp *resourceDetectionProcessor) processProfiles(_ context.Context, ld pprofile.Profiles) (pprofile.Profiles, error) {
	snap, _ := rdp.current.Load().(*resourceSnapshot)
	rl := ld.ResourceProfiles()
	for i := 0; i < rl.Len(); i++ {
		rss := rl.At(i)
		if snap != nil {
			rss.SetSchemaUrl(internal.MergeSchemaURL(rss.SchemaUrl(), snap.schemaURL))
			res := rss.Resource()
			internal.MergeResource(res, snap.res, rdp.override)
		}
	}
	return ld, nil
}
