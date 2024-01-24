// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package sharedcomponent exposes util functionality for receivers and exporters
// that need to share state between different signal types instances such as net.Listener or os.File.
//
// This package was copied from collector/internal/sharedcomponent.
package sharedcomponent
