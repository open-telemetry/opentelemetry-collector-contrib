// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pprofreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pprofreceiver"

import (
	"bufio"
	"bytes"
	"context"
	"runtime"
	"runtime/pprof"
	"time"

	"github.com/google/pprof/profile"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/xconsumer"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/xreceiver"
	"go.uber.org/zap"

	pproftranslator "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/pprof"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pprofreceiver/internal/metadata"
)

func NewFactory() receiver.Factory {
	return xreceiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		xreceiver.WithProfiles(createProfilesReceiver, metadata.ProfilesStability))
}

func createDefaultConfig() component.Config {
	return &Config{
		CollectionInterval: 10 * time.Second,
	}
}

func createProfilesReceiver(
	_ context.Context,
	set receiver.Settings,
	cfg component.Config,
	c xconsumer.Profiles,
) (xreceiver.Profiles, error) {
	return &pprofReceiver{
		consumer:          c,
		telemetrySettings: set.TelemetrySettings,
		config:            cfg.(*Config),
		done:              make(chan struct{}),
	}, nil
}

type pprofReceiver struct {
	consumer          xconsumer.Profiles
	telemetrySettings component.TelemetrySettings
	config            *Config
	done              chan struct{}
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

func (r *pprofReceiver) Start(_ context.Context, _ component.Host) error {
	runtime.SetBlockProfileRate(r.config.BlockProfileFraction)
	runtime.SetMutexProfileFraction(r.config.MutexProfileFraction)

	go func() {
		timer := time.NewTicker(r.config.CollectionInterval)
		collecting := false
		payload := make([]byte, 0, 8096)
		buf := bytes.NewBuffer(payload)
		writer := bufio.NewWriter(buf)
		for {
			select {
			case <-r.done:
				pprof.StopCPUProfile()
				return
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
	}()

	return nil
}

func (r *pprofReceiver) Shutdown(_ context.Context) error {
	close(r.done)
	return nil
}
