// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ctxprofilestack // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxprofilestack"

import (
	"go.opentelemetry.io/collector/pdata/pprofile"
)

const (
	Name   = "profilestack"
	DocRef = "https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/pkg/ottl/contexts/ottlprofilestack"
)

type Context interface {
	GetProfileStack() pprofile.Stack
	GetProfilesDictionary() pprofile.ProfilesDictionary
}
