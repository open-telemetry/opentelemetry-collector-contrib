// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package loadbalancingexporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitialResolution(t *testing.T) {
	// prepare
	provided := []string{"endpoint-2", "endpoint-1"}
	res, err := newStaticResolver(provided)
	require.NoError(t, err)

	// test
	var resolved []string
	res.onChange(func(endpoints []string) {
		resolved = endpoints
	})
	require.NoError(t, res.start(context.Background()))
	defer func() {
		require.NoError(t, res.shutdown(context.Background()))
	}()

	// verify
	expected := []string{"endpoint-1", "endpoint-2"}
	assert.Equal(t, expected, resolved)
}

func TestResolvedOnlyOnce(t *testing.T) {
	// prepare
	expected := []string{"endpoint-1", "endpoint-2"}
	res, err := newStaticResolver(expected)
	require.NoError(t, err)

	counter := 0
	res.onChange(func(endpoints []string) {
		counter++
	})

	// test
	require.NoError(t, res.start(context.Background()))
	defer func() {
		require.NoError(t, res.shutdown(context.Background()))
	}()
	resolved, err := res.resolve(context.Background()) // second resolution, should be noop

	// verify
	assert.NoError(t, err)
	assert.Equal(t, 1, counter)
	assert.Equal(t, expected, resolved)
}

func TestFailOnMissingEndpoints(t *testing.T) {
	// prepare
	var expected []string

	// test
	res, err := newStaticResolver(expected)

	// verify
	assert.Equal(t, errNoEndpoints, err)
	assert.Nil(t, res)
}
