// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkametricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver"

import (
	"context"
	"fmt"
	"regexp"
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

type topicScraperFranz struct {
	adm *kadm.Client
	cl  *kgo.Client

	settings    receiver.Settings
	topicFilter *regexp.Regexp
	config      Config
	mb          *metadata.MetricsBuilder
}

func (s *topicScraperFranz) start(_ context.Context, _ component.Host) error {
	s.mb = metadata.NewMetricsBuilder(s.config.MetricsBuilderConfig, s.settings)
	return nil
}

func (s *topicScraperFranz) shutdown(context.Context) error {
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

func (s *topicScraperFranz) ensureClients(ctx context.Context) error {
	if s.adm != nil && s.cl != nil {
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

func (s *topicScraperFranz) scrape(ctx context.Context) (pmetric.Metrics, error) {
	if err := s.ensureClients(ctx); err != nil {
		return pmetric.Metrics{}, err
	}

	scrapeErrs := scrapererror.ScrapeErrors{}
	now := pcommon.NewTimestampFromTime(time.Now())

	// 1) list topics (with metadata details)
	td, err := s.adm.ListTopics(ctx)
	if err != nil {
		s.settings.Logger.Error("franz-go: ListTopics failed", zap.Error(err))
		return pmetric.Metrics{}, fmt.Errorf("franz-go: ListTopics failed: %w", err)
	}

	// filter topic names first
	var matched []string
	for name := range td {
		if s.topicFilter.MatchString(name) {
			matched = append(matched, name)
		}
	}

	// 2) offsets for matched topics (newest & oldest)
	endOffs, err := s.adm.ListEndOffsets(ctx, matched...)
	if err != nil {
		scrapeErrs.Add(fmt.Errorf("franz-go: ListEndOffsets failed: %w", err))
	}
	startOffs, err := s.adm.ListStartOffsets(ctx, matched...)
	if err != nil {
		scrapeErrs.Add(fmt.Errorf("franz-go: ListStartOffsets failed: %w", err))
	}

	// 3) per-topic configs & replication factor
	if s.config.Metrics.KafkaTopicLogRetentionPeriod.Enabled ||
		s.config.Metrics.KafkaTopicLogRetentionSize.Enabled ||
		s.config.Metrics.KafkaTopicMinInsyncReplicas.Enabled ||
		s.config.Metrics.KafkaTopicReplicationFactor.Enabled {
		// replication factor: derive from first partition's replica count
		if s.config.Metrics.KafkaTopicReplicationFactor.Enabled {
			for _, topic := range matched {
				if det, ok := td[topic]; ok {
					var rf int
					for _, pd := range det.Partitions {
						rf = len(pd.Replicas)
						break // first partition is enough; RF should be consistent
					}
					if rf > 0 {
						s.mb.RecordKafkaTopicReplicationFactorDataPoint(now, int64(rf), topic)
					}
				}
			}
		}

		rcs, derr := s.adm.DescribeTopicConfigs(ctx, matched...)
		if derr != nil {
			s.settings.Logger.Warn("franz-go: DescribeTopicConfigs failed", zap.Error(derr))
			scrapeErrs.AddPartial(len(matched), fmt.Errorf("DescribeTopicConfigs: %w", derr))
		} else {
			for _, topic := range matched {
				rc, _ := rcs.On(topic, nil)
				if rc.Err != nil {
					scrapeErrs.AddPartial(1, fmt.Errorf("topic %s: %w", topic, rc.Err))
					continue
				}
				for _, kv := range rc.Configs {
					switch kv.Key {
					case minInsyncReplicas:
						if s.config.Metrics.KafkaTopicMinInsyncReplicas.Enabled {
							if v, err := strconv.Atoi(kv.MaybeValue()); err == nil {
								s.mb.RecordKafkaTopicMinInsyncReplicasDataPoint(now, int64(v), topic)
							} else {
								scrapeErrs.AddPartial(1, fmt.Errorf("topic %s: parse %s=%q: %w", topic, minInsyncReplicas, kv.MaybeValue(), err))
							}
						}
					case retentionMs:
						if s.config.Metrics.KafkaTopicLogRetentionPeriod.Enabled {
							if v, err := strconv.Atoi(kv.MaybeValue()); err == nil {
								// seconds = ms / 1000
								s.mb.RecordKafkaTopicLogRetentionPeriodDataPoint(now, int64(v/1000), topic)
							} else {
								scrapeErrs.AddPartial(1, fmt.Errorf("topic %s: parse %s=%q: %w", topic, retentionMs, kv.MaybeValue(), err))
							}
						}
					case retentionBytes:
						if s.config.Metrics.KafkaTopicLogRetentionSize.Enabled {
							if v, err := strconv.Atoi(kv.MaybeValue()); err == nil {
								s.mb.RecordKafkaTopicLogRetentionSizeDataPoint(now, int64(v), topic)
							} else {
								scrapeErrs.AddPartial(1, fmt.Errorf("topic %s: parse %s=%q: %w", topic, retentionBytes, kv.MaybeValue(), err))
							}
						}
					}
				}
			}
		}
	}

	// 4) per-topic partitions & per-partition metrics
	for _, topic := range matched {
		det, ok := td[topic]
		if !ok {
			continue
		}
		// partitions count
		s.mb.RecordKafkaTopicPartitionsDataPoint(now, int64(len(det.Partitions)), topic)

		// iterate partitions without copying large structs
		for pid := range det.Partitions {
			pd := det.Partitions[pid]

			// replicas
			if s.config.Metrics.KafkaPartitionReplicas.Enabled {
				s.mb.RecordKafkaPartitionReplicasDataPoint(now, int64(len(pd.Replicas)), topic, int64(pid))
			}
			// in-sync replicas
			if s.config.Metrics.KafkaPartitionReplicasInSync.Enabled {
				s.mb.RecordKafkaPartitionReplicasInSyncDataPoint(now, int64(len(pd.ISR)), topic, int64(pid))
			}

			// offsets: newest/current and oldest (use .Offset)
			if or, ok := endOffs.Lookup(topic, pid); ok && or.Err == nil {
				s.mb.RecordKafkaPartitionCurrentOffsetDataPoint(now, or.Offset, topic, int64(pid))
			} else if ok && or.Err != nil {
				scrapeErrs.AddPartial(1, fmt.Errorf("topic %s partition %d: end offset error: %w", topic, pid, or.Err))
			}

			if or, ok := startOffs.Lookup(topic, pid); ok && or.Err == nil {
				s.mb.RecordKafkaPartitionOldestOffsetDataPoint(now, or.Offset, topic, int64(pid))
			} else if ok && or.Err != nil {
				scrapeErrs.AddPartial(1, fmt.Errorf("topic %s partition %d: start offset error: %w", topic, pid, or.Err))
			}
		}
	}

	rb := s.mb.NewResourceBuilder()
	rb.SetKafkaClusterAlias(s.config.ClusterAlias)
	return s.mb.Emit(metadata.WithResource(rb.Emit())), scrapeErrs.Combine()
}

// Factory helper for franz-go path (selected via feature gate elsewhere).
func createTopicsScraperFranz(_ context.Context, cfg Config, settings receiver.Settings) (scraper.Metrics, error) {
	topicFilter, err := regexp.Compile(cfg.TopicMatch)
	if err != nil {
		return nil, fmt.Errorf("failed to compile topic filter: %w", err)
	}
	s := &topicScraperFranz{
		settings:    settings,
		topicFilter: topicFilter,
		config:      cfg,
	}
	return scraper.NewMetrics(
		s.scrape,
		scraper.WithStart(s.start),
		scraper.WithShutdown(s.shutdown),
	)
}
