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

package azure

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMockProvider_Metadata(t *testing.T) {
	p := MockProvider{}
	p.On("Metadata").Return(&ComputeMetadata{Name: "foo"}, nil)
	metadata, err := p.Metadata(context.Background())
	require.NoError(t, err)
	require.NotNil(t, metadata)
	assert.Equal(t, "foo", metadata.Name)
}
