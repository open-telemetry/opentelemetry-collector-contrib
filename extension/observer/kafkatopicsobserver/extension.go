// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkatopicsobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/kafkatopicsobserver"
import (
	"context"
	"fmt"
	"regexp"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/endpointswatcher"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka"
)

var (
	_ extension.Extension = (*kafkaTopicsObserver)(nil)
	_ observer.Observable = (*kafkaTopicsObserver)(nil)
)

type kafkaTopicsObserver struct {
	*endpointswatcher.EndpointsWatcher
	logger           *zap.Logger
	config           *Config
	doneChan         chan struct{}
	cancelKafkaAdmin func()
	once             *sync.Once
	topics           []string
	kafkaAdmin       sarama.ClusterAdmin
	mu               sync.Mutex
}

func newObserver(logger *zap.Logger, config *Config) (extension.Extension, error) {
	kCtx, cancel := context.WithCancel(context.Background())

	admin, err := createKafkaClusterAdmin(kCtx, *config)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("could not create kafka cluster admin: %w", err)
	}

	d := &kafkaTopicsObserver{
		logger:           logger,
		config:           config,
		once:             &sync.Once{},
		topics:           []string{},
		cancelKafkaAdmin: cancel,
		kafkaAdmin:       admin,
		doneChan:         make(chan struct{}),
	}
	d.EndpointsWatcher = endpointswatcher.New(d, time.Second, logger)
	return d, nil
}

func (k *kafkaTopicsObserver) ListEndpoints() []observer.Endpoint {
	// mutex is used in order to test the extensions without "race detected during execution of test" error
	k.mu.Lock()
	defer k.mu.Unlock()
	endpoints := make([]observer.Endpoint, 0, len(k.topics))
	for _, topic := range k.topics {
		details := &observer.KafkaTopic{}
		endpoint := observer.Endpoint{
			ID:      observer.EndpointID(topic),
			Target:  topic,
			Details: details,
		}
		endpoints = append(endpoints, endpoint)
	}
	return endpoints
}

func (k *kafkaTopicsObserver) Start(_ context.Context, _ component.Host) error {
	k.once.Do(
		func() {
			go func() {
				topicsRefreshTicker := time.NewTicker(k.config.TopicsSyncInterval)
				defer topicsRefreshTicker.Stop()
				for {
					select {
					case <-k.doneChan:
						err := k.kafkaAdmin.Close()
						if err != nil {
							k.logger.Error("failed to close kafka cluster admin", zap.Error(err))
						} else {
							k.logger.Info("kafka cluster admin closed")
						}
						return
					case <-topicsRefreshTicker.C:
						// Collect all available topics
						topics, err := k.kafkaAdmin.ListTopics()
						if err != nil {
							k.logger.Error("failed to list topics: ", zap.Error(err))
							continue
						}
						var topicNames []string
						for topic := range topics {
							topicNames = append(topicNames, topic)
						}
						// Filter topics
						var subTopics []string
						reg, _ := regexp.Compile(k.config.TopicRegex)
						for _, t := range topicNames {
							if reg.MatchString(t) {
								subTopics = append(subTopics, t)
							}
						}
						// mutex is used in order to test the extensions without "race detected during execution of test" error
						k.mu.Lock()
						k.topics = subTopics
						k.mu.Unlock()
					}
				}
			}()
		})
	return nil
}

func (k *kafkaTopicsObserver) Shutdown(_ context.Context) error {
	k.StopListAndWatch()
	close(k.doneChan)
	k.cancelKafkaAdmin()
	return nil
}

var createKafkaClusterAdmin = func(ctx context.Context, config Config) (sarama.ClusterAdmin, error) {
	saramaConfig := sarama.NewConfig()

	var err error
	if config.ResolveCanonicalBootstrapServersOnly {
		saramaConfig.Net.ResolveCanonicalBootstrapServers = true
	}
	if config.ProtocolVersion != "" {
		if saramaConfig.Version, err = sarama.ParseKafkaVersion(config.ProtocolVersion); err != nil {
			return nil, err
		}
	}
	if err := kafka.ConfigureAuthentication(ctx, config.Authentication, saramaConfig); err != nil {
		return nil, err
	}
	return sarama.NewClusterAdmin(config.Brokers, saramaConfig)
}
