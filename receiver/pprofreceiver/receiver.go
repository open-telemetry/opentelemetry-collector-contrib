// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pprofreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pprofreceiver"
import (
	"bufio"
	"bytes"
	"context"
	"io"
	"net/http"
	"runtime"
	"runtime/pprof"
	"time"

	"github.com/google/pprof/profile"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/xconsumer"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	pproftranslator "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/pprof"
)

type pprofReceiver struct {
	consumer          xconsumer.Profiles
	telemetrySettings component.TelemetrySettings
	config            *Config
	done              chan struct{}
	errGrp            errgroup.Group
	client            *http.Client
}

func (r *pprofReceiver) convert(p []byte) (pprofile.Profiles, error) {
	parsed, err := profile.ParseData(p)
	if err != nil {
		r.telemetrySettings.Logger.Debug("failed to parse pprof profile", zap.Error(err))
		return pprofile.Profiles{}, err
	}
	pp, err := pproftranslator.ConvertPprofToPprofile(parsed)
	if err != nil {
		r.telemetrySettings.Logger.Debug("failed to convert pprof profile", zap.Error(err))
		return pprofile.Profiles{}, err
	}
	return *pp, nil
}

func (r *pprofReceiver) Start(ctx context.Context, host component.Host) error {
	runtime.SetBlockProfileRate(r.config.BlockProfileFraction)
	runtime.SetMutexProfileFraction(r.config.MutexProfileFraction)

	if r.config.Endpoint != "" {
		var err error
		r.client, err = r.config.ToClient(ctx, host, r.telemetrySettings)
		if err != nil {
			return err
		}
		r.errGrp.Go(r.runRemote)
	} else {
		r.errGrp.Go(r.run)
	}

	return nil
}

func (r *pprofReceiver) runRemote() error {
	timer := time.NewTicker(r.config.CollectionInterval)
	for {
		select {
		case <-r.done:
			return nil
		case <-timer.C:
			resp, err := r.client.Get(r.config.Endpoint)
			if err != nil {
				r.telemetrySettings.Logger.Error("error requesting profile", zap.Error(err), zap.String("endpoint", r.config.Endpoint))
				continue
			}
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				r.telemetrySettings.Logger.Error("failed to read HTTP response", zap.Error(err))
				continue
			}
			_ = resp.Body.Close()
			pp, err := r.convert(body)
			if err != nil {
				r.telemetrySettings.Logger.Error("failed to convert pprof profile", zap.Error(err))
				continue
			}
			err = r.consumer.ConsumeProfiles(context.Background(), pp)
			if err != nil {
				r.telemetrySettings.Logger.Error("failed to ingest pprof profile", zap.Error(err))
			}
		}
	}
}

func (r *pprofReceiver) run() error {
	timer := time.NewTicker(r.config.CollectionInterval)
	collecting := false
	buf := bytes.NewBuffer(make([]byte, 0, 8096))
	writer := bufio.NewWriter(buf)
	for {
		select {
		case <-r.done:
			pprof.StopCPUProfile()
			return nil
		case <-timer.C:
			if collecting {
				pprof.StopCPUProfile()
				_ = writer.Flush()
				collecting = false
				pp, err := r.convert(buf.Bytes())
				buf.Reset()
				if err != nil {
					r.telemetrySettings.Logger.Error("error processing profile", zap.Error(err))
				} else {
					err = r.consumer.ConsumeProfiles(context.Background(), pp)
					if err != nil {
						r.telemetrySettings.Logger.Error("failed to ingest pprof profile", zap.Error(err))
					}
				}
			} else {
				err := pprof.StartCPUProfile(writer)
				if err != nil {
					r.telemetrySettings.Logger.Error("failed to start CPU profile", zap.Error(err))
				}
				collecting = true
			}
		}
	}
}

func (r *pprofReceiver) Shutdown(_ context.Context) error {
	close(r.done)
	if r.client != nil {
		r.client.CloseIdleConnections()
	}
	return r.errGrp.Wait()
}
