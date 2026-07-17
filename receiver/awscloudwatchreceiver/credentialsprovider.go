// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscloudwatchreceiver"

import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"go.opentelemetry.io/collector/component"
)

// awsCredentialsProvider is implemented by the awscredentialsprovider extension. It is
// declared locally (structural typing) to avoid a module dependency on the extension.
type awsCredentialsProvider interface {
	GetCredentialsProvider() aws.CredentialsProvider
}

// resolveCredentialsProvider returns the credentials provider from the
// awscredentialsprovider extension referenced by id, or nil when none is configured.
func resolveCredentialsProvider(host component.Host, id *component.ID) (aws.CredentialsProvider, error) {
	if id == nil {
		return nil, nil
	}
	ext, ok := host.GetExtensions()[*id]
	if !ok {
		return nil, fmt.Errorf("unknown credentials_provider extension %q", id)
	}
	provider, ok := ext.(awsCredentialsProvider)
	if !ok {
		return nil, fmt.Errorf("extension %q does not provide AWS credentials", id)
	}
	return provider.GetCredentialsProvider(), nil
}
