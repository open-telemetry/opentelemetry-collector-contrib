// Copyright The OpenTelemetry Authors
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

//go:build windows
// +build windows

package windows

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEventCloseWhenAlreadyClosed(t *testing.T) {
	event := NewEvent(0)
	err := event.Close()
	require.NoError(t, err)
}

func TestEventCloseSyscallFailure(t *testing.T) {
	event := NewEvent(5)
	closeProc = SimpleMockProc(0, 0, ErrorNotSupported)
	err := event.Close()
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to close event handle")
}

func TestEventCloseSuccess(t *testing.T) {
	event := NewEvent(5)
	closeProc = SimpleMockProc(1, 0, ErrorSuccess)
	err := event.Close()
	require.NoError(t, err)
	require.Equal(t, uintptr(0), event.handle)
}
