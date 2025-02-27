// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkatopicsobserver

import (
	"context"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/kafkatopicsobserver/internal/metadata"
)

type MockClusterAdmin struct {
	sarama.ClusterAdmin
	mock.Mock
}

func (m *MockClusterAdmin) ListTopics() (map[string]sarama.TopicDetail, error) {
	args := m.Called()
	return args.Get(0).(map[string]sarama.TopicDetail), args.Error(1)
}

func (m *MockClusterAdmin) Close() error {
	return m.Called().Error(0)
}

func TestCollectEndpointsDefaultConfig(t *testing.T) {
	factory := NewFactory()
	mockAdmin := &MockClusterAdmin{}
	// Override the createKafkaClusterAdmin function to return the mock admin
	originalCreateKafkaClusterAdmin := createKafkaClusterAdmin
	createKafkaClusterAdmin = func(_ context.Context, _ Config) (sarama.ClusterAdmin, error) {
		return mockAdmin, nil
	}

	ext, err := newObserver(zap.NewNop(), factory.CreateDefaultConfig().(*Config))
	require.NoError(t, err)
	require.NotNil(t, ext)

	obvs, ok := ext.(*kafkaTopicsObserver)
	require.True(t, ok)
	kEndpoints := obvs.ListEndpoints()

	want := []observer.Endpoint{}
	createKafkaClusterAdmin = originalCreateKafkaClusterAdmin
	require.Equal(t, want, kEndpoints)
}

func TestCollectEndpointsAllConfigSettings(t *testing.T) {
	mockAdmin := &MockClusterAdmin{}
	// During first check new topics matching the regex are detected
	mockAdmin.On("ListTopics").Return(map[string]sarama.TopicDetail{
		"topic1": {},
		"topic2": {},
	}, nil).Once()

	// During the second check only one new topic which doesn't match a regex is detected
	mockAdmin.On("ListTopics").Return(map[string]sarama.TopicDetail{
		"topic1": {},
		"topic2": {},
		"topics": {},
	}, nil).Once()

	// During the third check one topic matching a regex is detected
	mockAdmin.On("ListTopics").Return(map[string]sarama.TopicDetail{
		"topic1": {},
		"topics": {},
	}, nil).Once()
	mockAdmin.On("Close").Return(nil).Once()

	// Override the createKafkaClusterAdmin function to return the mock admin
	originalCreateKafkaClusterAdmin := createKafkaClusterAdmin
	createKafkaClusterAdmin = func(_ context.Context, _ Config) (sarama.ClusterAdmin, error) {
		return mockAdmin, nil
	}

	extAllSettings := loadConfig(t, component.NewIDWithName(metadata.Type, "all_settings"))
	ext, err := newObserver(zap.NewNop(), extAllSettings)
	require.NoError(t, err)
	require.NotNil(t, ext)

	obvs := ext.(*kafkaTopicsObserver)
	err = obvs.Start(context.Background(), nil)
	require.NoError(t, err)
	time.Sleep(6 * time.Second)

	kEndpoints := obvs.ListEndpoints()
	want := []observer.Endpoint{
		{
			ID:      "topic1",
			Target:  "topic1",
			Details: &observer.KafkaTopic{},
		},
		{
			ID:      "topic2",
			Target:  "topic2",
			Details: &observer.KafkaTopic{},
		},
	}
	require.ElementsMatch(t, want, kEndpoints)

	time.Sleep(5 * time.Second)
	kEndpoints = obvs.ListEndpoints()
	require.ElementsMatch(t, want, kEndpoints)

	time.Sleep(5 * time.Second)
	kEndpoints = obvs.ListEndpoints()
	want = []observer.Endpoint{
		{
			ID:      "topic1",
			Target:  "topic1",
			Details: &observer.KafkaTopic{},
		},
	}
	require.ElementsMatch(t, want, kEndpoints)

	err = obvs.Shutdown(context.Background())
	require.NoError(t, err)
	createKafkaClusterAdmin = originalCreateKafkaClusterAdmin
}
