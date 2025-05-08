// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkatopicsobserver

import (
	"cmp"
	"context"
	"slices"
	"testing"

	"github.com/IBM/sarama"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/kafkatopicsobserver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/kafka/configkafka"
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
	mockAdmin.On("ListTopics").Return(map[string]sarama.TopicDetail{"abc": {}, "def": {}}, nil)
	mockAdmin.On("Close").Return(nil).Once()

	ext, err := newObserver(
		zap.NewNop(),
		factory.CreateDefaultConfig().(*Config),
		func(context.Context, configkafka.ClientConfig) (sarama.ClusterAdmin, error) {
			return mockAdmin, nil
		},
	)
	require.NoError(t, err)
	require.NotNil(t, ext)

	err = ext.Start(context.Background(), componenttest.NewNopHost())
	assert.NoError(t, err)

	err = ext.Shutdown(context.Background())
	assert.NoError(t, err)
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
	}, nil)
	mockAdmin.On("Close").Return(nil).Once()

	// Override the createKafkaClusterAdmin function to return the mock admin
	extAllSettings := loadConfig(t, component.NewIDWithName(metadata.Type, "all_settings"))
	ext, err := newObserver(
		zap.NewNop(), extAllSettings,
		func(context.Context, configkafka.ClientConfig) (sarama.ClusterAdmin, error) {
			return mockAdmin, nil
		},
	)
	require.NoError(t, err)
	require.NotNil(t, ext)

	err = ext.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		err := ext.Shutdown(context.Background())
		assert.NoError(t, err)
	}()

	ops := make(channelNotifier, 1) // buffer for the initial op
	ext.(*kafkaTopicsObserver).ListAndWatch(ops)

	assert.Equal(t, notifyOp{
		op: "add",
		endpoints: []observer.Endpoint{{
			ID:      "topic1",
			Target:  "topic1",
			Details: &observer.KafkaTopic{},
		}, {
			ID:      "topic2",
			Target:  "topic2",
			Details: &observer.KafkaTopic{},
		}},
	}, <-ops)

	assert.Equal(t, notifyOp{
		op: "remove",
		endpoints: []observer.Endpoint{{
			ID:      "topic2",
			Target:  "topic2",
			Details: &observer.KafkaTopic{},
		}},
	}, <-ops)
}

type channelNotifier chan notifyOp

func (ch channelNotifier) ID() observer.NotifyID {
	return "channel-notifier"
}

func (ch channelNotifier) OnAdd(added []observer.Endpoint) {
	ch.addOp("add", added)
}

func (ch channelNotifier) OnRemove(removed []observer.Endpoint) {
	ch.addOp("remove", removed)
}

func (ch channelNotifier) OnChange(changed []observer.Endpoint) {
	ch.addOp("remove", changed)
}

func (ch channelNotifier) addOp(op string, endpoints []observer.Endpoint) {
	ch <- notifyOp{
		op: op,
		endpoints: slices.SortedFunc(
			slices.Values(endpoints),
			func(a, b observer.Endpoint) int {
				return cmp.Compare(a.ID, b.ID)
			},
		),
	}
}

type notifyOp struct {
	op        string // add, remove, change
	endpoints []observer.Endpoint
}
