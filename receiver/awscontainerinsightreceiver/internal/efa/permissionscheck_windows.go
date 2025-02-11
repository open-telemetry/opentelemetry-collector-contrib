// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package efa // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/efa"

import (
	"errors"
	"os"
)

func checkPermissions(_ os.FileInfo) error {
	return errors.New("not implemented on Windows")
}
