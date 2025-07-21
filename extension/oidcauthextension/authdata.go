// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oidcauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/oidcauthextension"

import "go.opentelemetry.io/collector/client"

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
		e, ok := a.claims[name]
		if ok {
			return e
		}

		return nil
	}
}

func (a *authData) GetAttributeNames() []string {
	attributeNames := []string{"subject", "membership", "raw"}
	for n := range a.claims {
		attributeNames = append(attributeNames, n)
	}

	return attributeNames
}
