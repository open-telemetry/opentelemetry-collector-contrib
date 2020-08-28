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

package translation

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTTLMapData(t *testing.T) {
	m := newTTLMapData(10)
	require.Nil(t, m.get("foo"))
	m.put("bob", "xyz", 2)
	m.sweep(12)
	require.NotNil(t, m.get("bob"))
	m.sweep(13)
	require.Nil(t, m.get("bob"))
}

// disabled due to 5 second run time
func DisabledTestNewMap(t *testing.T) {
	m := newTTLMap(2, 1)
	m.start()
	m.put("bob", "xyz")
	time.Sleep(time.Second)
	v := m.get("bob")
	require.NotNil(t, v)
	time.Sleep(4 * time.Second)
	v = m.get("bob")
	require.Nil(t, v)
}
