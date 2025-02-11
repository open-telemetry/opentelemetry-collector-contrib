// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package kubeletutil // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/stores/kubeletutil"

import (
	"errors"
	"os"
)

func checkPodResourcesSocketPermissions(_ os.FileInfo) error {
	return errors.New("not implemented on Windows")
}
