// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kubeletstatsreceiver

import (
	"context"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/kubelet"
)

func TestRunnable(t *testing.T) {
	consumer := &fakeConsumer{}
	r := newRunnable(
		context.Background(),
		consumer,
		&fakeRestClient{},
		zap.NewNop(),
	)
	err := r.Setup()
	require.NoError(t, err)
	err = r.Run()
	require.NoError(t, err)
	require.Equal(t, 19, len(consumer.mds))
}

type fakeConsumer struct {
	mds []consumerdata.MetricsData
}

func (c *fakeConsumer) ConsumeMetricsData(
	ctx context.Context,
	md consumerdata.MetricsData,
) error {
	c.mds = append(c.mds, md)
	return nil
}

var _ kubelet.RestClient = (*fakeRestClient)(nil)

type fakeRestClient struct{}

func (f *fakeRestClient) StatsSummary() ([]byte, error) {
	return ioutil.ReadFile("testdata/stats-summary.json")
}

func (f *fakeRestClient) Pods() ([]byte, error) {
	return ioutil.ReadFile("testdata/pods.json")
}
