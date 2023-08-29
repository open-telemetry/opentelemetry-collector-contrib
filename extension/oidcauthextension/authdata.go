// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oidcauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/oidcauthextension"

import "go.opentelemetry.io/collector/client"

var _ client.AuthData = (*authData)(nil)

type authData struct {
	raw        string
	subject    string
	membership []string
}

func (a *authData) GetAttribute(name string) interface{} {
	switch name {
	case "subject":
		return a.subject
	case "membership":
		return a.membership
	case "raw":
		return a.raw
	default:
		return nil
	}
}

func (*authData) GetAttributeNames() []string {
	return []string{"subject", "membership", "raw"}
}
