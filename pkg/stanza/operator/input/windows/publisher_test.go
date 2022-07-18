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

func TestPublisherOpenPreexisting(t *testing.T) {
	publisher := Publisher{handle: 5}
	err := publisher.Open("")
	require.Error(t, err)
	require.Contains(t, err.Error(), "publisher handle is already open")
}

func TestPublisherOpenInvalidUTF8(t *testing.T) {
	publisher := NewPublisher()
	invalidUTF8 := "\u0000"
	err := publisher.Open(invalidUTF8)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to convert provider to utf16")
}

func TestPublisherOpenSyscallFailure(t *testing.T) {
	publisher := NewPublisher()
	provider := "provider"
	openPublisherMetadataProc = SimpleMockProc(0, 0, ErrorNotSupported)
	err := publisher.Open(provider)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to open publisher handle")
}

func TestPublisherOpenSuccess(t *testing.T) {
	publisher := NewPublisher()
	provider := "provider"
	openPublisherMetadataProc = SimpleMockProc(5, 0, ErrorSuccess)
	err := publisher.Open(provider)
	require.NoError(t, err)
	require.Equal(t, uintptr(5), publisher.handle)
}

func TestPublisherCloseWhenAlreadyClosed(t *testing.T) {
	publisher := NewPublisher()
	err := publisher.Close()
	require.NoError(t, err)
}

func TestPublisherCloseSyscallFailure(t *testing.T) {
	publisher := Publisher{handle: 5}
	closeProc = SimpleMockProc(0, 0, ErrorNotSupported)
	err := publisher.Close()
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to close publisher")
}

func TestPublisherCloseSuccess(t *testing.T) {
	publisher := Publisher{handle: 5}
	closeProc = SimpleMockProc(1, 0, ErrorSuccess)
	err := publisher.Close()
	require.NoError(t, err)
	require.Equal(t, uintptr(0), publisher.handle)
}
