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
	"errors"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/kubelet"
)

const dataLen = 19

func TestRunnable(t *testing.T) {
	consumer := &fakeConsumer{}
	r := newRunnable(
		context.Background(),
		"",
		consumer,
		&fakeRestClient{},
		zap.NewNop(),
	)
	err := r.Setup()
	require.NoError(t, err)
	err = r.Run()
	require.NoError(t, err)
	require.Equal(t, dataLen, len(consumer.mds))
}

func TestClientErrors(t *testing.T) {
	tests := []struct {
		name    string
		fail    bool
		numLogs int
	}{
		{"no error", false, 0},
		{"rest error", true, 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			core, observedLogs := observer.New(zap.ErrorLevel)
			r := newRunnable(
				context.Background(),
				"",
				&fakeConsumer{},
				&fakeRestClient{fail: test.fail},
				zap.New(core),
			)
			err := r.Setup()
			require.NoError(t, err)
			err = r.Run()
			require.NoError(t, err)
			require.Equal(t, test.numLogs, observedLogs.Len())
		})
	}
}

func TestConsumerErrors(t *testing.T) {
	tests := []struct {
		name    string
		fail    bool
		numLogs int
	}{
		{"no error", false, 0},
		{"consume error", true, dataLen},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			core, observedLogs := observer.New(zap.ErrorLevel)
			r := newRunnable(
				context.Background(),
				"",
				&fakeConsumer{fail: test.fail},
				&fakeRestClient{},
				zap.New(core),
			)
			err := r.Setup()
			require.NoError(t, err)
			err = r.Run()
			require.NoError(t, err)
			require.Equal(t, test.numLogs, observedLogs.Len())
		})
	}
}

type fakeConsumer struct {
	fail bool
	mds  []consumerdata.MetricsData
}

func (c *fakeConsumer) ConsumeMetricsData(
	ctx context.Context,
	md consumerdata.MetricsData,
) error {
	if c.fail {
		return errors.New("")
	}
	c.mds = append(c.mds, md)
	return nil
}

var _ kubelet.RestClient = (*fakeRestClient)(nil)

type fakeRestClient struct {
	fail bool
}

func (f *fakeRestClient) StatsSummary() ([]byte, error) {
	if f.fail {
		return nil, errors.New("")
	}
	return ioutil.ReadFile("testdata/stats-summary.json")
}

func (f *fakeRestClient) Pods() ([]byte, error) {
	if f.fail {
		return nil, errors.New("")
	}
	return ioutil.ReadFile("testdata/pods.json")
}
