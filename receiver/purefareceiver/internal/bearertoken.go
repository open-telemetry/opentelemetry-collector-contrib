// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/purefareceiver/internal"

import (
	"context"
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configauth"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/bearertokenauthextension"
)

func RetrieveBearerToken(authCfg configauth.Authentication, extensions map[component.ID]component.Component) (string, error) {
	auth, err := authCfg.GetGRPCClientAuthenticator(context.Background(), extensions)
	if err != nil {
		return "", err
	}

	// only bearer token auth is supported for now
	if _, ok := auth.(*bearertokenauthextension.BearerTokenAuth); !ok {
		return "", fmt.Errorf("only bearer token auth extension is supported at this moment, but got %T", auth)
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
