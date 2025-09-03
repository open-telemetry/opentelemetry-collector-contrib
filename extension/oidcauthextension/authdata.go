// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oidcauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/oidcauthextension"

import (
	"strings"

	"go.opentelemetry.io/collector/client"
)

const claimsPrefix = "claims."

var _ client.AuthData = (*authData)(nil)

type authData struct {
	raw    string
	claims map[string]any

	subject    string
	membership []string
}

func (a *authData) GetAttribute(name string) any {
	switch name {
	case "subject":
		return a.subject
	case "membership":
		return a.membership
	case "raw":
		return a.raw
	default:
		prefixed := strings.HasPrefix(name, claimsPrefix)
		if !prefixed {
			// If the name does not start with the claims prefix, we return nil.
			// This ensures not mixing custom JWT claims with hard defined attributes.
			return nil
		}

		name = strings.TrimPrefix(name, claimsPrefix)
		e, ok := a.claims[name]
		if ok {
			return e
		}

		return nil
	}
}

func (a *authData) GetAttributeNames() []string {
	attributeNames := []string{"subject", "membership", "raw"}
	for claimName := range a.claims {
		attributeNames = append(attributeNames, claimsPrefix+claimName)
	}

	return attributeNames
}
