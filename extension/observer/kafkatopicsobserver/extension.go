// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkatopicsobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/kafkatopicsobserver"
import (
	"context"
	"fmt"
	"regexp"
	"sync"

	"github.com/IBM/sarama"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/endpointswatcher"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka/configkafka"
)

var (
	_ extension.Extension = (*kafkaTopicsObserver)(nil)
	_ observer.Observable = (*kafkaTopicsObserver)(nil)
)

type kafkaTopicsObserver struct {
	*endpointswatcher.EndpointsWatcher
	logger *zap.Logger
	config *Config

	adminClient      sarama.ClusterAdmin
	cancelKafkaAdmin func()
}

func newObserver(
	logger *zap.Logger,
	config *Config,
	newAdminClusterClient func(context.Context, configkafka.ClientConfig) (sarama.ClusterAdmin, error),
) (extension.Extension, error) {
	topicRegexp, err := regexp.Compile(config.TopicRegex)
	if err != nil {
		return nil, fmt.Errorf("failed to compile topic regex: %w", err)
	}

	kCtx, cancel := context.WithCancel(context.Background())
	adminClient, err := newAdminClusterClient(kCtx, config.ClientConfig)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("could not create kafka cluster admin: %w", err)
	}

	o := &kafkaTopicsObserver{
		logger:           logger,
		config:           config,
		cancelKafkaAdmin: cancel,
		adminClient:      adminClient,
	}
	o.EndpointsWatcher = endpointswatcher.New(
		&kafkaTopicsEndpointsLister{o: o, topicRegexp: topicRegexp},
		config.TopicsSyncInterval,
		logger,
	)
	return o, nil
}

func (k *kafkaTopicsObserver) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (k *kafkaTopicsObserver) Shutdown(_ context.Context) error {
	k.StopListAndWatch()
	k.cancelKafkaAdmin()
	err := k.adminClient.Close()
	if err != nil {
		return fmt.Errorf("failed to close kafka cluster admin client: %w", err)
	}
	k.logger.Info("kafka cluster admin client closed")
	return nil
}

type kafkaTopicsEndpointsLister struct {
	o           *kafkaTopicsObserver
	topicRegexp *regexp.Regexp

	mu     sync.Mutex
	topics []string
}

func (k *kafkaTopicsEndpointsLister) ListEndpoints() []observer.Endpoint {
	topics, err := k.listMatchingTopics()
	if err != nil {
		k.o.logger.Error("failed to list topics, using cached list", zap.Error(err))
		// Use the previously cached list of topics.
		k.mu.Lock()
		topics = k.topics
		k.mu.Unlock()
	} else {
		// Cache the new list of topics.
		k.mu.Lock()
		k.topics = topics
		k.mu.Unlock()
	}
	endpoints := make([]observer.Endpoint, len(topics))
	for i, topic := range topics {
		details := &observer.KafkaTopic{}
		endpoints[i] = observer.Endpoint{
			ID:      observer.EndpointID(topic),
			Target:  topic,
			Details: details,
		}
	}
	return endpoints
}

func (k *kafkaTopicsEndpointsLister) listMatchingTopics() ([]string, error) {
	// Collect all available topics
	topics, err := k.o.adminClient.ListTopics()
	if err != nil {
		return nil, err
	}

	// Filter topics
	var matchingTopics []string
	for topic := range topics {
		if k.topicRegexp.MatchString(topic) {
			matchingTopics = append(matchingTopics, topic)
		}
	}
	return matchingTopics, nil
}
