// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

var _ ClusterNameProvider = (*StringClusterProvider)(nil)

type StringClusterProvider string

func (p StringClusterProvider) ClusterName(context.Context) (string, error) { return string(p), nil }

var _ ClusterNameProvider = (*ErrorClusterProvider)(nil)

type ErrorClusterProvider string

func (p ErrorClusterProvider) ClusterName(context.Context) (string, error) {
	return "", errors.New(string(p))
}

func TestChainCluster(t *testing.T) {
	tests := []struct {
		name         string
		providers    map[string]ClusterNameProvider
		priorityList []string

		buildErr string

		clusterName string
		queryErr    string
	}{
		{
			name: "missing provider in priority list",
			providers: map[string]ClusterNameProvider{
				"p1": StringClusterProvider("p1ClusterName"),
				"p2": ErrorClusterProvider("errP2"),
			},
			priorityList: []string{"p1", "p2", "p3"},

			buildErr: "\"p3\" source is not available in providers",
		},
		{
			name: "all providers fail",
			providers: map[string]ClusterNameProvider{
				"p1": ErrorClusterProvider("errP1"),
				"p2": ErrorClusterProvider("errP2"),
				"p3": StringClusterProvider("p3ClusterName"),
			},
			priorityList: []string{"p1", "p2"},

			queryErr: "no cluster name provider was available",
		},
		{
			name: "no providers fail",
			providers: map[string]ClusterNameProvider{
				"p1": StringClusterProvider("p1ClusterName"),
				"p2": StringClusterProvider("p2ClusterName"),
				"p3": StringClusterProvider("p3ClusterName"),
			},
			priorityList: []string{"p1", "p2", "p3"},

			clusterName: "p1ClusterName",
		},
		{
			name: "some providers fail",
			providers: map[string]ClusterNameProvider{
				"p1": ErrorClusterProvider("p1Err"),
				"p2": StringClusterProvider("p2ClusterName"),
				"p3": ErrorClusterProvider("p3Err"),
			},
			priorityList: []string{"p1", "p2", "p3"},

			clusterName: "p2ClusterName",
		},
	}

	for _, testInstance := range tests {
		t.Run(testInstance.name, func(t *testing.T) {
			provider, err := ChainCluster(zap.NewNop(), testInstance.providers, testInstance.priorityList)
			if err != nil || testInstance.buildErr != "" {
				assert.EqualError(t, err, testInstance.buildErr)
				return
			}

			clusterName, err := provider.ClusterName(context.Background())
			if err != nil || testInstance.queryErr != "" {
				assert.EqualError(t, err, testInstance.queryErr)
			} else {
				assert.Equal(t, testInstance.clusterName, clusterName)
			}
		})
	}
}
