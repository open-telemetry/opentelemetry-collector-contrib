// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkametricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver"

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver/internal/metadata"
)

type consumerScraperFranz struct {
	adm *kadm.Client
	cl  *kgo.Client

	settings    receiver.Settings
	groupFilter *regexp.Regexp
	topicFilter *regexp.Regexp
	config      Config
	mb          *metadata.MetricsBuilder
	host        component.Host
}

func (s *consumerScraperFranz) start(_ context.Context, host component.Host) error {
	s.mb = metadata.NewMetricsBuilder(s.config.MetricsBuilderConfig, s.settings)
	s.host = host
	return nil
}

func (s *consumerScraperFranz) shutdown(_ context.Context) error {
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

func (s *consumerScraperFranz) ensureClients(ctx context.Context) error {
	if s.adm != nil && s.cl != nil {
		return nil
	}
	adm, cl, err := kafka.NewFranzClusterAdminClient(ctx, s.host, s.config.ClientConfig, s.settings.Logger)
	if err != nil {
		return fmt.Errorf("failed to create franz-go admin client: %w", err)
	}
	s.adm = adm
	s.cl = cl
	return nil
}

func (s *consumerScraperFranz) scrape(ctx context.Context) (pmetric.Metrics, error) {
	if err := s.ensureClients(ctx); err != nil {
		return pmetric.Metrics{}, err
	}

	lgs, err := s.adm.ListGroupsByType(ctx, []string{"classic", "consumer"})
	if err != nil {
		return pmetric.Metrics{}, fmt.Errorf("franz-go: ListGroups failed: %w", err)
	}

	var matchedGrpIDs []string
	for _, g := range lgs {
		if s.groupFilter.MatchString(g.Group) {
			matchedGrpIDs = append(matchedGrpIDs, g.Group)
		}
	}

	dgls, err := s.adm.Lag(ctx, matchedGrpIDs...)
	if err != nil {
		return pmetric.Metrics{}, fmt.Errorf("franz-go: Lag failed: %w", err)
	}

	scrapeErrs := scrapererror.ScrapeErrors{}
	now := pcommon.NewTimestampFromTime(time.Now())
	for group := range dgls {
		dgl := dgls[group]
		if dgl.DescribeErr != nil {
			scrapeErrs.AddPartial(1, fmt.Errorf("franz-go: returned error from describing the group. group=%s, error=%w", group, dgl.DescribeErr))
			continue
		}
		if dgl.FetchErr != nil {
			scrapeErrs.AddPartial(1, fmt.Errorf("franz-go: returned error from fetching offsets. group=%s, error=%w", group, dgl.FetchErr))
			continue
		}
		s.mb.RecordKafkaConsumerGroupMembersDataPoint(now, int64(len(dgl.Members)), group)
		for topic := range dgl.Lag {
			if !s.topicFilter.MatchString(topic) {
				continue
			}
			gmls := dgl.Lag[topic]
			var isConsumed bool
			var offsetSum int64
			var lagSum int64
			for partition := range gmls {
				gml := gmls[partition]
				if gml.Err != nil {
					scrapeErrs.AddPartial(1, fmt.Errorf("franz-go: returned either the commit error, or the list end offsets error. group=%s, topic=%s, partition=%d, error=%w", group, topic, partition, gml.Err))
					continue
				}
				if gml.Commit.At != -1 {
					isConsumed = true
					offsetSum += gml.Commit.At
					s.mb.RecordKafkaConsumerGroupOffsetDataPoint(now, gml.Commit.At, group, topic, int64(partition))
					if gml.Lag <= 0 {
						s.mb.RecordKafkaConsumerGroupLagDataPoint(now, 0, group, topic, int64(partition))
					} else {
						lagSum += gml.Lag
						s.mb.RecordKafkaConsumerGroupLagDataPoint(now, gml.Lag, group, topic, int64(partition))
					}
				}
			}
			if isConsumed {
				s.mb.RecordKafkaConsumerGroupOffsetSumDataPoint(now, offsetSum, group, topic)
				s.mb.RecordKafkaConsumerGroupLagSumDataPoint(now, lagSum, group, topic)
			}
		}
	}

	rb := s.mb.NewResourceBuilder()
	rb.SetKafkaClusterAlias(s.config.ClusterAlias)

	return s.mb.Emit(metadata.WithResource(rb.Emit())), scrapeErrs.Combine()
}

// Factory helper for franz-go path (selected under the feature gate later).
func createConsumerScraperFranz(_ context.Context, cfg Config, settings receiver.Settings) (scraper.Metrics, error) {
	groupFilter, err := regexp.Compile(cfg.GroupMatch)
	if err != nil {
		return nil, fmt.Errorf("failed to compile group_match: %w", err)
	}
	topicFilter, err := regexp.Compile(cfg.TopicMatch)
	if err != nil {
		return nil, fmt.Errorf("failed to compile topic filter: %w", err)
	}
	s := &consumerScraperFranz{
		settings:    settings,
		groupFilter: groupFilter,
		topicFilter: topicFilter,
		config:      cfg,
	}
	return scraper.NewMetrics(
		s.scrape,
		scraper.WithStart(s.start),
		scraper.WithShutdown(s.shutdown),
	)
}
