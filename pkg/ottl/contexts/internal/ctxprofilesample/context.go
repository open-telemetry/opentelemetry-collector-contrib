// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ctxprofilesample // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxprofilesample"

import (
	"go.opentelemetry.io/collector/pdata/pprofile"
)

const (
	Name   = "profilesample"
	DocRef = "https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/pkg/ottl/contexts/ottlprofilesample"
)

type Context interface {
	GetProfileSample() pprofile.Sample
	GetProfilesDictionary() pprofile.ProfilesDictionary
}
