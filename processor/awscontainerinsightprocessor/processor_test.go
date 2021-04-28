// Copyright  OpenTelemetry Authors
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

package awscontainerinsightprocessor

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/collector/consumer/pdata"
)

type MockStore struct {
	RefreshCount int
	wg           sync.WaitGroup
}

func (m *MockStore) Decorate(metric pdata.ResourceMetrics, kubernetesBlob map[string]interface{}) bool {
	return true
}

func (m *MockStore) RefreshTick(ctx context.Context) {
	m.RefreshCount++
	if m.RefreshCount <= 2 {
		m.wg.Done()
	}
}

func (m *MockStore) currentRefreshCount() int {
	m.wg.Wait()
	return m.RefreshCount
}

type LongRunningMockStore struct {
	RefreshCount int
	wg           sync.WaitGroup
}

func (m *LongRunningMockStore) Decorate(metric pdata.ResourceMetrics, kubernetesBlob map[string]interface{}) bool {
	return true
}

func (m *LongRunningMockStore) RefreshTick(ctx context.Context) {
	m.RefreshCount++

	if m.RefreshCount == 2 {
		waitForever := make(chan bool)
		// to simulate a long running store
		go func() {
			m.wg.Done() // to signal the second execution of RefreshTick
			<-waitForever
		}()

		<-ctx.Done()
	}
}

func TestInitAndShutdown(t *testing.T) {
	originalHostName := os.Getenv("HOST_NAME")
	originalHostIP := os.Getenv("HOST_IP")
	os.Setenv("HOST_NAME", "host_name")
	os.Setenv("HOST_IP", "host_ip")

	processor := &awscontainerinsightprocessor{
		storeRefreshMinInterval: time.Millisecond,
		storeRefreshTimeout:     2 * time.Millisecond,
		storeRefreshStoppedC:    make(chan bool),
	}
	mockStore := &MockStore{}
	mockStore.wg.Add(2)
	processor.stores = append(processor.stores, mockStore)
	processor.init()

	//block until refreshTick() executes twice
	if mockStore.currentRefreshCount() != 2 {
		t.Fail()
	}

	processor.Shutdown(context.Background())
	//wait two refresh intervals for the channel to fire
	time.Sleep(2 * processor.storeRefreshMinInterval)

	select {
	case <-processor.storeRefreshStoppedC:
	default:
		t.Error("Channel is not closed")
	}

	os.Setenv("HOST_NAME", originalHostName)
	os.Setenv("HOST_IP", originalHostIP)
}

func TestInitAndShutdownWithLongRunningStore(t *testing.T) {
	originalHostName := os.Getenv("HOST_NAME")
	originalHostIP := os.Getenv("HOST_IP")
	os.Setenv("HOST_NAME", "host_name")
	os.Setenv("HOST_IP", "host_ip")

	processor := &awscontainerinsightprocessor{
		storeRefreshMinInterval: time.Millisecond,
		storeRefreshTimeout:     2 * time.Millisecond,
		storeRefreshStoppedC:    make(chan bool),
	}
	mockStore := &LongRunningMockStore{}
	mockStore.wg.Add(1)
	processor.stores = append(processor.stores, mockStore)
	processor.init()

	//wait until the refreshTick() to execute the second time
	mockStore.wg.Wait()

	processor.Shutdown(context.Background())
	//wait for the channel to fire
	time.Sleep(2 * processor.storeRefreshTimeout)

	select {
	case <-processor.storeRefreshStoppedC:
	default:
		t.Error("Channel is not closed")
	}

	os.Setenv("HOST_NAME", originalHostName)
	os.Setenv("HOST_IP", originalHostIP)
}
