// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package attrs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/attrs"

import (
	"fmt"
	"os"
)

func (r *Resolver) addOwnerInfo(file *os.File, attributes map[string]any) (err error) {
	return fmt.Errorf("addOwnerInfo it's not implemented for windows: %w", err)
}
