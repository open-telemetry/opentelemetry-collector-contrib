// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ctxprofile // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxprofile"
import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"
)

const (
	Name   = "profile"
	DocRef = "https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/pkg/ottl/contexts/ottlprofile"
)

type Context interface {
	AttributeIndices() pcommon.Int32Slice
	GetProfile() pprofile.Profile
	GetProfilesDictionary() pprofile.ProfilesDictionary
}
