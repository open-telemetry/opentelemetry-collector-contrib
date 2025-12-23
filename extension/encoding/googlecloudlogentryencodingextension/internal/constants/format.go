// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package constants // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/googlecloudlogentryencodingextension/internal/constants"

const (
	FormatAuditLog          = "auditlog"
	FormatVPCFlowLog        = "vpcflow"
	FormatLoadBalancerLog   = "load-balancer"
	FormatProxyNLBLog       = "proxy-nlb"
	FormatDNSQueryLog       = "dns-query"
	FormatPassthroughNLBLog = "passthrough-nlb"

	FormatIdentificationTag = "encoding.format"

	// GCP-specific format prefixes
	GCPFormatPrefix            = "gcp."
	GCPFormatAuditLog          = GCPFormatPrefix + FormatAuditLog
	GCPFormatVPCFlowLog        = GCPFormatPrefix + FormatVPCFlowLog
	GCPFormatLoadBalancerLog   = GCPFormatPrefix + FormatLoadBalancerLog
	GCPFormatProxyNLBLog       = GCPFormatPrefix + FormatProxyNLBLog
	GCPFormatDNSQueryLog       = GCPFormatPrefix + FormatDNSQueryLog
	GCPFormatPassthroughNLBLog = GCPFormatPrefix + FormatPassthroughNLBLog
)
