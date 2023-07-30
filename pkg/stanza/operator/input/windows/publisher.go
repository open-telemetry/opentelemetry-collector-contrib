// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package windows // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/windows"

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
		return fmt.Errorf("failed to convert provider to utf16: %w", err)
	}

	handle, err := evtOpenPublisherMetadata(0, utf16, nil, 0, 0)
	if err != nil {
		return fmt.Errorf("failed to open publisher handle: %w", err)
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
		return fmt.Errorf("failed to close publisher: %w", err)
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
