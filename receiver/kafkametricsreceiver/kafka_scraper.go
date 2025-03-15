// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkametricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver"

import (
	"context"
	"fmt"

	"github.com/IBM/sarama"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver/internal/metadata"
)

// kafkaScraper is intended to be embedded in the more specific scraper types
// for implementing the Start and Shutdown methods, and managing the clients.
type kafkaScraper struct {
	config                      Config
	settings                    receiver.Settings
	mb                          *metadata.MetricsBuilder
	newSaramaClient             newSaramaClientFunc
	newSaramaClusterAdminClient newSaramaClusterAdminClientFunc

	client      sarama.Client
	adminClient sarama.ClusterAdmin
}

func (s *kafkaScraper) start(ctx context.Context, _ component.Host) error {
	client, err := s.newSaramaClient(ctx, s.config.ClientConfig)
	if err != nil {
		return fmt.Errorf("failed to create Kafka client: %w", err)
	}
	s.client = client

	s.adminClient, err = s.newSaramaClusterAdminClient(client)
	if err != nil {
		client.Close()
		return fmt.Errorf("failed to create Kafka cluster admin client: %w", err)
	}
	return nil
}

func (s *kafkaScraper) shutdown(context.Context) error {
	if s.client != nil {
		if err := s.client.Close(); err != nil {
			return fmt.Errorf("failed to close client: %w", err)
		}
		// It is not necessary to close the admin client,
		// it's just a wrapper around the base client.
	}
	return nil
}
