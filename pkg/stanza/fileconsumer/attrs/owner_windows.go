// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package attrs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/attrs"

import (
	"errors"
	"os"
)

func (r *Resolver) addOwnerInfo(_ *os.File, _ map[string]any) error {
	return errors.New("owner info not implemented for windows")
}
