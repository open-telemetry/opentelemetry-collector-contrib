// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkametricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver"

import (
	"context"
	"errors"
	"strconv"

	"github.com/IBM/sarama"
	"github.com/stretchr/testify/mock"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/kafka/configkafka"
)

const (
	testBroker            = "test_broker"
	testClusterAlias      = "test_cluster"
	testGroup             = "test_group"
	testTopic             = "test_topic"
	testConsumerClient    = "test_consumer_client"
	testPartition         = 1
	testReplicationFactor = 2
	testMinInsyncReplicas = 1
	testLogRetentionBytes = -1
	testLogRetentionMs    = 86_400_000
	testLogRetentionHours = 168
)

var (
	testTopics     = []string{testTopic}
	testPartitions = []int32{1}
	testReplicas   = []int32{1}
	testBrokers    = make([]*sarama.Broker, 1)
)

func mockNewSaramaClient(context.Context, configkafka.ClientConfig) (sarama.Client, error) {
	return newMockClient(), nil
}

func mockNewClusterAdmin(sarama.Client) (sarama.ClusterAdmin, error) {
	return newMockClusterAdmin(), nil
}

type mockSaramaClient struct {
	mock.Mock
	sarama.Client

	close          error
	closed         bool
	brokers        []*sarama.Broker
	topics         []string
	partitions     []int32
	offset         int64
	replicas       []int32
	inSyncReplicas []int32
}

func (s *mockSaramaClient) Closed() bool {
	s.Called()
	return s.closed
}

func (s *mockSaramaClient) Close() error {
	s.Called()
	return s.close
}

func (s *mockSaramaClient) Brokers() []*sarama.Broker {
	s.Called()
	return s.brokers
}

func (s *mockSaramaClient) Topics() ([]string, error) {
	if s.topics != nil {
		return s.topics, nil
	}
	return nil, errors.New("mock topic error")
}

func (s *mockSaramaClient) Partitions(string) ([]int32, error) {
	if s.partitions != nil {
		return s.partitions, nil
	}
	return nil, errors.New("mock partition error")
}

func (s *mockSaramaClient) GetOffset(string, int32, int64) (int64, error) {
	if s.offset != -1 {
		return s.offset, nil
	}
	return s.offset, errors.New("mock offset error")
}

func (s *mockSaramaClient) Replicas(string, int32) ([]int32, error) {
	if s.replicas != nil {
		return s.replicas, nil
	}
	return nil, errors.New("mock replicas error")
}

func (s *mockSaramaClient) InSyncReplicas(string, int32) ([]int32, error) {
	if s.inSyncReplicas != nil {
		return s.inSyncReplicas, nil
	}
	return nil, errors.New("mock in sync replicas error")
}

func newMockClient() *mockSaramaClient {
	client := new(mockSaramaClient)
	client.close = nil
	r := sarama.NewBroker(testBroker)

	testBrokers[0] = r
	client.closed = false
	client.offset = 1
	client.brokers = testBrokers
	client.partitions = testPartitions
	client.topics = testTopics
	client.inSyncReplicas = testReplicas
	client.replicas = testReplicas

	return client
}

type mockClusterAdmin struct {
	mock.Mock
	sarama.ClusterAdmin
	brokerConfigs             []sarama.ConfigEntry
	topics                    map[string]sarama.TopicDetail
	consumerGroups            map[string]string
	consumerGroupDescriptions []*sarama.GroupDescription
	consumerGroupOffsets      *sarama.OffsetFetchResponse
}

func (s *mockClusterAdmin) ListTopics() (map[string]sarama.TopicDetail, error) {
	if s.topics == nil {
		return nil, errors.New("error getting topics")
	}
	return s.topics, nil
}

func (s *mockClusterAdmin) ListConsumerGroups() (map[string]string, error) {
	if s.consumerGroups == nil {
		return nil, errors.New("error getting consumer groups")
	}
	return s.consumerGroups, nil
}

func (s *mockClusterAdmin) DescribeConsumerGroups([]string) ([]*sarama.GroupDescription, error) {
	if s.consumerGroupDescriptions == nil {
		return nil, errors.New("error describing consumer groups")
	}
	return s.consumerGroupDescriptions, nil
}

func (s *mockClusterAdmin) ListConsumerGroupOffsets(string, map[string][]int32) (*sarama.OffsetFetchResponse, error) {
	if s.consumerGroupOffsets == nil {
		return nil, errors.New("mock consumer group offset error")
	}
	return s.consumerGroupOffsets, nil
}

func (s *mockClusterAdmin) DescribeConfig(cr sarama.ConfigResource) ([]sarama.ConfigEntry, error) {
	topicName := cr.Name
	if cr.Type == sarama.BrokerResource {
		return s.brokerConfigs, nil
	}
	if s.topics[topicName].ConfigEntries == nil {
		return nil, errors.New("no config entries found for topic")
	}
	configEntry := make([]sarama.ConfigEntry, 1)
	for name, entry := range s.topics[topicName].ConfigEntries {
		configEntry = append(configEntry, sarama.ConfigEntry{
			Name:  name,
			Value: *entry,
		})
	}
	return configEntry, nil
}

func newMockClusterAdmin() *mockClusterAdmin {
	clusterAdmin := new(mockClusterAdmin)
	r := make(map[string]string)
	r[testGroup] = testGroup
	clusterAdmin.consumerGroups = r

	strMinInsyncReplicas := strconv.Itoa(testMinInsyncReplicas)
	strLogRetentionMs := strconv.Itoa(testLogRetentionMs)
	strLogRetentionBytes := strconv.Itoa(testLogRetentionBytes)

	brokerConfigEntry := sarama.ConfigEntry{
		Name:  logRetentionHours,
		Value: strconv.Itoa(testLogRetentionHours),
	}
	configEntries := make([]sarama.ConfigEntry, 1)
	configEntries = append(configEntries, brokerConfigEntry)
	clusterAdmin.brokerConfigs = configEntries
	td := make(map[string]sarama.TopicDetail)
	td[testTopic] = sarama.TopicDetail{
		ReplicationFactor: testReplicationFactor,
		ConfigEntries: map[string]*string{
			minInsyncReplicas: &strMinInsyncReplicas,
			retentionMs:       &strLogRetentionMs,
			retentionBytes:    &strLogRetentionBytes,
		},
	}
	clusterAdmin.topics = td

	desc := sarama.GroupMemberDescription{
		ClientId: testConsumerClient,
	}
	gmd := make(map[string]*sarama.GroupMemberDescription)
	gmd[testConsumerClient] = &desc
	d := sarama.GroupDescription{
		GroupId: testGroup,
		Members: gmd,
	}
	gd := make([]*sarama.GroupDescription, 1)
	gd[0] = &d
	clusterAdmin.consumerGroupDescriptions = gd

	blocks := make(map[string]map[int32]*sarama.OffsetFetchResponseBlock)
	topicBlocks := make(map[int32]*sarama.OffsetFetchResponseBlock)
	block := sarama.OffsetFetchResponseBlock{
		Offset: 1,
	}
	topicBlocks[testPartition] = &block
	blocks[testTopic] = topicBlocks
	offsetRes := sarama.OffsetFetchResponse{
		Blocks: blocks,
	}
	clusterAdmin.consumerGroupOffsets = &offsetRes

	return clusterAdmin
}
