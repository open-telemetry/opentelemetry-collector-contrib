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

// +build windows

package windows

import (
	"fmt"
	"syscall"
)

// Publisher is a windows event metadata publisher.
type Publisher struct {
	handle uintptr
}

// Open will open the publisher handle using the supplied provider.
func (p *Publisher) Open(provider string) error {
	if p.handle != 0 {
		return fmt.Errorf("publisher handle is already open")
	}

	utf16, err := syscall.UTF16PtrFromString(provider)
	if err != nil {
		return fmt.Errorf("failed to convert provider to utf16: %s", err)
	}

	handle, err := evtOpenPublisherMetadata(0, utf16, nil, 0, 0)
	if err != nil {
		return fmt.Errorf("failed to open publisher handle: %s", err)
	}

	p.handle = handle
	return nil
}

// Close will close the publisher handle.
func (p *Publisher) Close() error {
	if p.handle == 0 {
		return nil
	}

	if err := evtClose(p.handle); err != nil {
		return fmt.Errorf("failed to close publisher: %s", err)
	}

	p.handle = 0
	return nil
}

// NewPublisher will create a new publisher with an empty handle.
func NewPublisher() Publisher {
	return Publisher{
		handle: 0,
	}
}
