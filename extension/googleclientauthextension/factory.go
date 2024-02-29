// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

package googleclientauthextension

import (
	"go.opentelemetry.io/collector/extension"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/extension/googleclientauthextension"
)

func NewFactory() extension.Factory {
	return googleclientauthextension.NewFactory()
}
