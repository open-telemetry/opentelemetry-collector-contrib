// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ctxprofilelocation // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxprofilelocation"

import "go.opentelemetry.io/collector/pdata/pprofile"

const (
	Name   = "profilelocation"
	DocRef = "https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/pkg/ottl/contexts/ottlprofilelocation"
)

type Context interface {
	GetProfileLocation() pprofile.Location
	GetProfilesDictionary() pprofile.ProfilesDictionary
}
