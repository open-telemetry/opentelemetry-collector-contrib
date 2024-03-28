// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

package googleclientauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/googleclientauthextension"

import (
	"go.opentelemetry.io/collector/extension"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/googleclientauthextension/internal/metadata"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/extension/googleclientauthextension"
)

func NewFactory() extension.Factory {
	return extension.NewFactory(
		metadata.Type,
		googleclientauthextension.CreateDefaultConfig,
		googleclientauthextension.CreateExtension,
		metadata.ExtensionStability,
	)
}
