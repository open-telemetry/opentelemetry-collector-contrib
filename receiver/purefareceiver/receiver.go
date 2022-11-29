// Copyright 2022 The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package purefareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/purefareceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/purefareceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/purefareceiver/internal/array"
)

var _ component.MetricsReceiver = (*purefaReceiver)(nil)

type scraperType string

const (
	arrayScraper       scraperType = "array"
	hostScraper        scraperType = "host"
	volumeScraper      scraperType = "volume"
	podsScraper        scraperType = "pods"
	directoriesScraper scraperType = "directories"
)

type purefaReceiver struct {
	cfg  *Config
	set  component.ReceiverCreateSettings
	next consumer.Metrics

	recvs map[scraperType]internal.Scraper
}

func newReceiver(cfg *Config, set component.ReceiverCreateSettings, next consumer.Metrics) *purefaReceiver {
	return &purefaReceiver{
		cfg:   cfg,
		set:   set,
		next:  next,
		recvs: make(map[scraperType]internal.Scraper),
	}
}

func (r *purefaReceiver) Start(ctx context.Context, host component.Host) error {
	r.recvs[arrayScraper] = array.NewScraper(ctx, r.set, r.next, r.cfg.Endpoint, r.cfg.Arrays, r.cfg.Settings.ReloadIntervals.Array)
	if err := r.recvs[arrayScraper].Start(ctx, host); err != nil {
		return err
	}

	return nil
}

func (r *purefaReceiver) Shutdown(ctx context.Context) error {
	var errs error
	for _, v := range r.recvs {
		err := v.Shutdown(ctx)
		if err != nil {
			errs = multierr.Append(errs, err)
		}
	}
	return errs
}
