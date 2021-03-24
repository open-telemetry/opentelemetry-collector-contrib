// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kafkametricsreceiver

import (
	"fmt"

	"github.com/Shopify/sarama"
	"github.com/stretchr/testify/mock"
)

const (
	testBroker = "test_broker"
)

var newSaramaClient = sarama.NewClient

var testTopics = []string{"test_topic"}
var testPartitions = []int32{1}
var testReplicas = []int32{1}
var testBrokers = make([]*sarama.Broker, 1)

func mockNewSaramaClient([]string, *sarama.Config) (sarama.Client, error) {
	return newMockClient(), nil
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
	return nil, fmt.Errorf("mock topic error")
}

func (s *mockSaramaClient) Partitions(string) ([]int32, error) {
	if s.partitions != nil {
		return s.partitions, nil
	}
	return nil, fmt.Errorf("mock partition error")
}

func (s *mockSaramaClient) GetOffset(string, int32, int64) (int64, error) {
	if s.offset != -1 {
		return s.offset, nil
	}
	return s.offset, fmt.Errorf("mock offset error")
}

func (s *mockSaramaClient) Replicas(string, int32) ([]int32, error) {
	if s.replicas != nil {
		return s.replicas, nil
	}
	return nil, fmt.Errorf("mock replicas error")
}

func (s *mockSaramaClient) InSyncReplicas(string, int32) ([]int32, error) {
	if s.inSyncReplicas != nil {
		return s.inSyncReplicas, nil
	}
	return nil, fmt.Errorf("mock in sync replicas error")
}

func newMockClient() *mockSaramaClient {
	client := new(mockSaramaClient)
	client.close = nil
	r := sarama.NewBroker(testBroker)
	testBrokers[0] = r

	client.closed = false
	client.offset = -1
	client.brokers = testBrokers
	client.partitions = testPartitions
	client.topics = testTopics
	client.inSyncReplicas = testReplicas
	client.replicas = testReplicas

	return client
}
