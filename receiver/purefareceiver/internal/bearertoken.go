// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/purefareceiver/internal"

import (
	"context"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configauth"
)

func RetrieveBearerToken(authCfg configauth.Config, extensions map[component.ID]component.Component) (string, error) {
	auth, err := authCfg.GetGRPCClientAuthenticator(context.Background(), extensions)
	if err != nil {
		return "", err
	}

	cr, err := auth.PerRPCCredentials()
	if err != nil {
		return "", err
	}

	headers, err := cr.GetRequestMetadata(context.Background(), "")
	if err != nil {
		return "", err
	}

	return strings.TrimPrefix(headers["authorization"], "Bearer "), nil
}
