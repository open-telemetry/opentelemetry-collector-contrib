// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package tracker

import (
	"context"
)

func (t *Tracker) PreConsume(ctx context.Context) {
}

func (t *Tracker) Rotate() {

}

// On windows, we close files immediately after reading becauase they cannot be moved while open.
func (t *Tracker) PostConsume() {
	// close open files and move them to closed fileset
	// move active fileset to open fileset
	// empty out active fileset
	t.openFiles.Reset(t.ActiveFiles()...)
	t.closePreviousFiles()
	t.activeFiles.Clear()
}
