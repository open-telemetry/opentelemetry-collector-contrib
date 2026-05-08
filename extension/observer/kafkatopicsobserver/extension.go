// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkatopicsobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/kafkatopicsobserver"
import (
	"context"
	"fmt"
	"regexp"
	"sync"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
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
	logger *zap.Logger
	config *Config

	client *kadm.Client
}

func newObserver(logger *zap.Logger, config *Config) (extension.Extension, error) {
	topicRegexp, err := regexp.Compile(config.TopicRegex)
	if err != nil {
		return nil, fmt.Errorf("failed to compile topic regex: %w", err)
	}

	o := &kafkaTopicsObserver{
		logger: logger,
		config: config,
	}
	o.EndpointsWatcher = endpointswatcher.New(
		&kafkaTopicsEndpointsLister{o: o, topicRegexp: topicRegexp},
		config.TopicsSyncInterval,
		logger,
	)
	return o, nil
}

func (k *kafkaTopicsObserver) Start(ctx context.Context, host component.Host) error {
	client, _, err := kafka.NewFranzClusterAdminClient(
		ctx, host, k.config.ClientConfig, k.logger,
		// Set metadata min age below the sync interval
		// so each sync sees a fresh topic list.
		kgo.MetadataMinAge(k.config.TopicsSyncInterval/2),
	)
	if err != nil {
		return fmt.Errorf("could not create kafka cluster admin client: %w", err)
	}
	k.client = client
	return nil
}

func (k *kafkaTopicsObserver) Shutdown(context.Context) error {
	k.StopListAndWatch()
	if k.client != nil {
		k.client.Close()
	}
	return nil
}

type kafkaTopicsEndpointsLister struct {
	o           *kafkaTopicsObserver
	topicRegexp *regexp.Regexp

	mu     sync.Mutex
	topics []string
}

func (k *kafkaTopicsEndpointsLister) ListEndpoints() []observer.Endpoint {
	topics, err := k.listMatchingTopics(context.Background())
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

func (k *kafkaTopicsEndpointsLister) listMatchingTopics(ctx context.Context) ([]string, error) {
	td, err := k.o.client.ListTopics(ctx)
	if err != nil {
		return nil, err
	}
	var matchingTopics []string
	for name := range td {
		if k.topicRegexp.MatchString(name) {
			matchingTopics = append(matchingTopics, name)
		}
	}
	return matchingTopics, nil
}
