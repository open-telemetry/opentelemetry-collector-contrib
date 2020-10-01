// Copyright 2020, OpenTelemetry Authors
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

package metadata

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/stretchr/testify/assert"
)

func TestGetHostIdentifier(t *testing.T) {
	s, _ := session.NewSession()
	metadata := NewMetadata(s, "")
	_, err := metadata.GetHostIdentifier()
	assert.NotNil(t, err)
}

func TestGetHostIdentifierWithContainerId(t *testing.T) {
	s, _ := session.NewSession()
	metadata := NewMetadata(s, "../testdata/mockcgroup")

	_, err := metadata.GetHostIdentifier()
	assert.Nil(t, err)
}

func TestGetHostIdentifierWithContainerIdErr(t *testing.T) {
	s, _ := session.NewSession()
	metadata := NewMetadata(s, "../testdata/mockcgroupWithErr")

	id, err := metadata.GetHostIdentifier()
	assert.Equal(t, "", id)
	assert.Nil(t, err)
}
