// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkametricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver"

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scrapererror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver/internal/metadata"
)

type brokerScraperFranz struct {
	// franz-go handles (lazy created on first scrape)
	adm *kadm.Client
	cl  *kgo.Client

	settings receiver.Settings
	config   Config
	mb       *metadata.MetricsBuilder
}

func (s *brokerScraperFranz) start(_ context.Context, _ component.Host) error {
	s.mb = metadata.NewMetricsBuilder(s.config.MetricsBuilderConfig, s.settings)
	return nil
}

func (s *brokerScraperFranz) shutdown(context.Context) error {
	if s.adm != nil {
		s.adm.Close()
		s.adm = nil
	}
	if s.cl != nil {
		s.cl.Close()
		s.cl = nil
	}
	return nil
}

func (s *brokerScraperFranz) ensureClients(ctx context.Context) error {
	if s.cl != nil && s.adm != nil {
		return nil
	}
	adm, cl, err := kafka.NewFranzClusterAdminClient(ctx, s.config.ClientConfig, s.settings.Logger)
	if err != nil {
		return fmt.Errorf("failed to create franz-go admin client: %w", err)
	}
	s.adm = adm
	s.cl = cl
	return nil
}

func (s *brokerScraperFranz) scrape(ctx context.Context) (pmetric.Metrics, error) {
	scrapeErrs := scrapererror.ScrapeErrors{}

	if err := s.ensureClients(ctx); err != nil {
		return pmetric.Metrics{}, err
	}

	now := pcommon.NewTimestampFromTime(time.Now())
	rb := s.mb.NewResourceBuilder()
	rb.SetKafkaClusterAlias(s.config.ClusterAlias)

	// ---- brokers count ----
	bdetails, err := s.adm.ListBrokers(ctx)
	if err != nil {
		// If we cannot list brokers, emit what we have (resource attrs) and return the error
		scrapeErrs.Add(err)
		return s.mb.Emit(metadata.WithResource(rb.Emit())), scrapeErrs.Combine()
	}
	brokerIDs := bdetails.NodeIDs()
	s.mb.RecordKafkaBrokersDataPoint(now, int64(len(brokerIDs)))

	// If log retention metric is disabled, we are done.
	if !s.config.Metrics.KafkaBrokerLogRetentionPeriod.Enabled {
		return s.mb.Emit(metadata.WithResource(rb.Emit())), scrapeErrs.Combine()
	}

	res, err := s.adm.DescribeBrokerConfigs(ctx, brokerIDs...)
	if err != nil {
		s.settings.Logger.Warn("franz-go: DescribeBrokerConfigs failed", zap.Error(err))
		scrapeErrs.AddPartial(len(brokerIDs), fmt.Errorf("DescribeBrokerConfigs: %w", err))
	}

	// Iterate the result and record the metric for each broker entry we can parse.
	for _, bid := range brokerIDs {
		bidStr := strconv.Itoa(int(bid))

		// Look up this broker's config set by resource name (broker id as string).
		cfg, _ := res.On(bidStr, nil) // fn can be nil to just return the entry
		if cfg.Err != nil {
			scrapeErrs.AddPartial(1, fmt.Errorf("broker %s: %w", bidStr, cfg.Err))
			continue
		}

		for _, kv := range cfg.Configs {
			// kadm.Config has Key and MaybeValue() for the string value.
			// We only care about log.retention.hours here.
			if kv.Key != logRetentionHours {
				continue
			}
			raw := kv.MaybeValue()
			hrs, convErr := strconv.Atoi(raw)
			if convErr != nil {
				scrapeErrs.AddPartial(1, fmt.Errorf("broker %s: cannot parse %s=%q: %w", bidStr, logRetentionHours, raw, convErr))
				continue
			}
			sec := int64(hrs) * 3600
			s.mb.RecordKafkaBrokerLogRetentionPeriodDataPoint(now, sec, bidStr)
		}
	}

	return s.mb.Emit(metadata.WithResource(rb.Emit())), scrapeErrs.Combine()
}

// factory for franz-go scraper (internal; selected via gate at the call site later)
func createBrokerScraperFranz(_ context.Context, cfg Config, settings receiver.Settings) (scraper.Metrics, error) {
	s := &brokerScraperFranz{
		settings: settings,
		config:   cfg,
	}
	return scraper.NewMetrics(
		s.scrape,
		scraper.WithStart(s.start),
		scraper.WithShutdown(s.shutdown),
	)
}
