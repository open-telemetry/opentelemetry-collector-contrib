// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package consul // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/consul"

import "go.opentelemetry.io/collector/config/configopaque"

// The struct requires no user-specified fields by default as consul agent's default
// configuration will be provided to the API client.
// See `consul.go#NewDetector` for more information.
type Config struct {

	// Address is the address of the Consul server
	Address string `mapstructure:"address"`

	// Datacenter to use. If not provided, the default agent datacenter is used.
	Datacenter string `mapstructure:"datacenter"`

	// Token is used to provide a per-request ACL token
	// which overrides the agent's default (empty) token.
	// Token or Tokenfile are only required if [Consul's ACL
	// System](https://www.consul.io/docs/security/acl/acl-system) is enabled.
	Token configopaque.String `mapstructure:"token"`

	// TokenFile is a file containing the current token to use for this client.
	// If provided it is read once at startup and never again.
	// Token or Tokenfile are only required if [Consul's ACL
	// System](https://www.consul.io/docs/security/acl/acl-system) is enabled.
	TokenFile string `mapstructure:"token_file"`

	// Namespace is the name of the namespace to send along for the request
	// when no other Namespace is present in the QueryOptions
	Namespace string `mapstructure:"namespace"`

	// Allowlist of [Consul
	// Metadata](https://www.consul.io/docs/agent/options#node_meta) keys to use as
	// resource attributes.
	MetaLabels map[string]interface{} `mapstructure:"meta"`
}
