// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package networkfirewall // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler/network-firewall-log"

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"time"

	gojson "github.com/goccy/go-json"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/constants"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler"
)

type networkFirewallLogUnmarshaler struct {
	buildInfo component.BuildInfo
}

func NewNetworkFirewallLogUnmarshaler(buildInfo component.BuildInfo) unmarshaler.AWSUnmarshaler {
	return &networkFirewallLogUnmarshaler{
		buildInfo: buildInfo,
	}
}

// See log fields: https://docs.aws.amazon.com/network-firewall/latest/developerguide/firewall-logging-contents.html.
type networkFirewallLog struct {
	FirewallName     string `json:"firewall_name"`
	AvailabilityZone string `json:"availability_zone"`
	EventTimestamp   string `json:"event_timestamp"`
	Event            struct {
		EventType string `json:"event_type"`
		FlowID    int64  `json:"flow_id"`
		Src       string `json:"src_ip"`
		SrcPort   int64  `json:"src_port"`
		Dest      string `json:"dest_ip"`
		DestPort  int64  `json:"dest_port"`
		Proto     string `json:"proto"`
		SNI       string `json:"sni"` // TLS SNI at event level
		Netflow   struct {
			Pkts   int64  `json:"pkts"`
			Bytes  int64  `json:"bytes"`
			Start  string `json:"start"`
			End    string `json:"end"`
			Age    int64  `json:"age"`
			MaxTTL int64  `json:"max_ttl"`
			MinTTL int64  `json:"min_ttl"`
			TxCnt  int64  `json:"tx_cnt"`
		} `json:"netflow"`
		Alert struct {
			Action      string `json:"action"`
			Signature   string `json:"signature"`
			SignatureID int64  `json:"signature_id"`
			Rev         int64  `json:"rev"`
			Category    string `json:"category"`
			Severity    int64  `json:"severity"`
			Gid         int64  `json:"gid"`
			Metadata    struct {
				AffectedProduct   []string `json:"affected_product"`
				AttackTarget      []string `json:"attack_target"`
				Deployment        []string `json:"deployment"`
				FormerCategory    []string `json:"former_category"`
				MalwareFamily     []string `json:"malware_family"`
				PerformanceImpact []string `json:"performance_impact"`
				SignatureSeverity []string `json:"signature_severity"`
				CreatedAt         []string `json:"created_at"`
				UpdatedAt         []string `json:"updated_at"`
			} `json:"metadata"`
		} `json:"alert"`
		RevocationCheck struct {
			LeafCertFpr string `json:"leaf_cert_fpr"`
			Action      string `json:"action"`
			Status      string `json:"status"`
		} `json:"revocation_check"`
		TLSError struct {
			ErrorMessage string `json:"error_message"`
		} `json:"tls_error"`
		TLS struct {
			Subject        string `json:"subject"`
			Issuer         string `json:"issuer"`
			SessionResumed *bool  `json:"session_resumed"`
		} `json:"tls"`
		HTTP struct {
			Hostname        string `json:"hostname"`
			URL             string `json:"url"`
			HTTPUserAgent   string `json:"http_user_agent"`
			HTTPContentType string `json:"http_content_type"`
			Cookie          string `json:"cookie"`
		} `json:"http"`
	} `json:"event"`
}

func (n *networkFirewallLogUnmarshaler) UnmarshalAWSLogs(reader io.Reader) (plog.Logs, error) {
	logs := plog.NewLogs()

	resourceLogs := logs.ResourceLogs().AppendEmpty()
	resourceLogs.Resource().Attributes().PutStr(
		string(conventions.CloudProviderKey),
		conventions.CloudProviderAWS.Value.AsString(),
	)

	scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()
	scopeLogs.Scope().SetName(metadata.ScopeName)
	scopeLogs.Scope().SetVersion(n.buildInfo.Version)
	scopeLogs.Scope().Attributes().PutStr(constants.FormatIdentificationTag, "aws."+constants.FormatNetworkFirewallLog)

	firewallName := ""
	availabilityZone := ""

	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		logLine := scanner.Bytes()

		var log networkFirewallLog
		if err := gojson.Unmarshal(logLine, &log); err != nil {
			return plog.Logs{}, fmt.Errorf("failed to unmarshal Network Firewall log: %w", err)
		}
		if log.FirewallName == "" {
			return plog.Logs{}, errors.New("invalid Network Firewall log: empty firewall_name field")
		}
		if firewallName == "" {
			firewallName = log.FirewallName
			availabilityZone = log.AvailabilityZone
		}
		if firewallName != log.FirewallName {
			return plog.Logs{}, fmt.Errorf(
				"unexpected: new firewall_name %q is different than previous one %q",
				log.FirewallName,
				firewallName,
			)
		}

		record := scopeLogs.LogRecords().AppendEmpty()
		if err := n.addNetworkFirewallLog(log, record); err != nil {
			return plog.Logs{}, err
		}
	}

	if err := setResourceAttributes(resourceLogs, firewallName, availabilityZone); err != nil {
		return plog.Logs{}, fmt.Errorf("failed to set resource attributes: %w", err)
	}

	return logs, nil
}

func setResourceAttributes(resourceLogs plog.ResourceLogs, firewallName, availabilityZone string) error {
	if firewallName == "" {
		return errors.New("firewall_name is required")
	}

	resourceLogs.Resource().Attributes().PutStr("aws.networkfirewall.name", firewallName)

	if availabilityZone != "" {
		resourceLogs.Resource().Attributes().PutStr(string(conventions.CloudAvailabilityZoneKey), availabilityZone)
	}

	return nil
}

func (*networkFirewallLogUnmarshaler) addNetworkFirewallLog(log networkFirewallLog, record plog.LogRecord) error {
	// Parse event timestamp
	timestamp, err := time.Parse(time.RFC3339, log.EventTimestamp)
	if err != nil {
		return fmt.Errorf("failed to parse event_timestamp %q: %w", log.EventTimestamp, err)
	}
	record.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))

	putStr := func(name, value string) {
		if value != "" {
			record.Attributes().PutStr(name, value)
		}
	}

	putInt := func(name string, value int64) {
		if value != 0 {
			record.Attributes().PutInt(name, value)
		}
	}

	putBool := func(name string, value bool) {
		record.Attributes().PutBool(name, value)
	}

	putStrSlice := func(name string, values []string) {
		if len(values) > 0 {
			slice := record.Attributes().PutEmptySlice(name)
			for _, v := range values {
				slice.AppendEmpty().SetStr(v)
			}
		}
	}

	// Add common event-level fields
	if log.Event.EventType != "" {
		putStr("aws.networkfirewall.event.type", log.Event.EventType)
	}
	if log.Event.FlowID != 0 {
		putInt("aws.networkfirewall.flow_id", log.Event.FlowID)
	}

	// Add network fields
	if log.Event.Src != "" {
		putStr(string(conventions.SourceAddressKey), log.Event.Src)
	}
	if log.Event.SrcPort != 0 {
		putInt(string(conventions.SourcePortKey), log.Event.SrcPort)
	}
	if log.Event.Dest != "" {
		putStr(string(conventions.DestinationAddressKey), log.Event.Dest)
	}
	if log.Event.DestPort != 0 {
		putInt(string(conventions.DestinationPortKey), log.Event.DestPort)
	}
	if log.Event.Proto != "" {
		putStr("network.transport", log.Event.Proto)
	}

	// Add netflow fields if present (Flow event type)
	if log.Event.Netflow.Pkts != 0 {
		putInt("aws.networkfirewall.netflow.packets", log.Event.Netflow.Pkts)
	}
	if log.Event.Netflow.Bytes != 0 {
		putInt("aws.networkfirewall.netflow.bytes", log.Event.Netflow.Bytes)
	}
	if log.Event.Netflow.Start != "" {
		putStr("aws.networkfirewall.netflow.start", log.Event.Netflow.Start)
	}
	if log.Event.Netflow.End != "" {
		putStr("aws.networkfirewall.netflow.end", log.Event.Netflow.End)
	}
	if log.Event.Netflow.Age != 0 {
		putInt("aws.networkfirewall.netflow.age", log.Event.Netflow.Age)
	}
	if log.Event.Netflow.MaxTTL != 0 {
		putInt("aws.networkfirewall.netflow.max_ttl", log.Event.Netflow.MaxTTL)
	}
	if log.Event.Netflow.MinTTL != 0 {
		putInt("aws.networkfirewall.netflow.min_ttl", log.Event.Netflow.MinTTL)
	}
	if log.Event.Netflow.TxCnt != 0 {
		putInt("aws.networkfirewall.netflow.transaction.count", log.Event.Netflow.TxCnt)
	}

	// Add alert fields if present (Alert event type)
	if log.Event.Alert.Action != "" {
		putStr("aws.networkfirewall.alert.action", log.Event.Alert.Action)
	}
	if log.Event.Alert.Signature != "" {
		putStr("aws.networkfirewall.alert.signature", log.Event.Alert.Signature)
	}
	if log.Event.Alert.SignatureID != 0 {
		putInt("aws.networkfirewall.alert.signature_id", log.Event.Alert.SignatureID)
	}
	if log.Event.Alert.Rev != 0 {
		putInt("aws.networkfirewall.alert.rev", log.Event.Alert.Rev)
	}
	if log.Event.Alert.Category != "" {
		putStr("aws.networkfirewall.alert.category", log.Event.Alert.Category)
	}
	if log.Event.Alert.Severity != 0 {
		putInt("aws.networkfirewall.alert.severity", log.Event.Alert.Severity)
	}
	if log.Event.Alert.Gid != 0 {
		putInt("aws.networkfirewall.alert.gid", log.Event.Alert.Gid)
	}

	// Add alert metadata if present
	if len(log.Event.Alert.Metadata.AffectedProduct) > 0 {
		putStrSlice("aws.networkfirewall.alert.metadata.affected_product", log.Event.Alert.Metadata.AffectedProduct)
	}
	if len(log.Event.Alert.Metadata.AttackTarget) > 0 {
		putStrSlice("aws.networkfirewall.alert.metadata.attack_target", log.Event.Alert.Metadata.AttackTarget)
	}
	if len(log.Event.Alert.Metadata.Deployment) > 0 {
		putStrSlice("aws.networkfirewall.alert.metadata.deployment", log.Event.Alert.Metadata.Deployment)
	}
	if len(log.Event.Alert.Metadata.FormerCategory) > 0 {
		putStrSlice("aws.networkfirewall.alert.metadata.former_category", log.Event.Alert.Metadata.FormerCategory)
	}
	if len(log.Event.Alert.Metadata.MalwareFamily) > 0 {
		putStrSlice("aws.networkfirewall.alert.metadata.malware_family", log.Event.Alert.Metadata.MalwareFamily)
	}
	if len(log.Event.Alert.Metadata.PerformanceImpact) > 0 {
		putStrSlice("aws.networkfirewall.alert.metadata.performance_impact", log.Event.Alert.Metadata.PerformanceImpact)
	}
	if len(log.Event.Alert.Metadata.SignatureSeverity) > 0 {
		putStrSlice("aws.networkfirewall.alert.metadata.signature_severity", log.Event.Alert.Metadata.SignatureSeverity)
	}
	if len(log.Event.Alert.Metadata.CreatedAt) > 0 {
		putStrSlice("aws.networkfirewall.alert.metadata.created_at", log.Event.Alert.Metadata.CreatedAt)
	}
	if len(log.Event.Alert.Metadata.UpdatedAt) > 0 {
		putStrSlice("aws.networkfirewall.alert.metadata.updated_at", log.Event.Alert.Metadata.UpdatedAt)
	}

	// SNI can be at event level
	if log.Event.SNI != "" {
		putStr(string(conventions.ServerAddressKey), log.Event.SNI)
	}

	// Revocation check fields (check each field individually)
	if log.Event.RevocationCheck.LeafCertFpr != "" {
		putStr("aws.networkfirewall.tls.revocation_check.leaf_cert_fpr", log.Event.RevocationCheck.LeafCertFpr)
	}
	if log.Event.RevocationCheck.Action != "" {
		putStr("aws.networkfirewall.tls.revocation_check.action", log.Event.RevocationCheck.Action)
	}
	if log.Event.RevocationCheck.Status != "" {
		putStr("aws.networkfirewall.tls.revocation_check.status", log.Event.RevocationCheck.Status)
	}

	// TLS error (check for existence)
	if log.Event.TLSError.ErrorMessage != "" {
		putStr("aws.networkfirewall.tls.error.message", log.Event.TLSError.ErrorMessage)
	}

	// TLS details
	if log.Event.TLS.Subject != "" {
		putStr("tls.client.subject", log.Event.TLS.Subject)
	}
	if log.Event.TLS.Issuer != "" {
		putStr("tls.client.issuer", log.Event.TLS.Issuer)
	}

	// TLS session resumed (only if field is present in JSON)
	if log.Event.TLS.SessionResumed != nil {
		putBool("tls.resumed", *log.Event.TLS.SessionResumed)
	}

	// Set HTTP fields if present
	if log.Event.HTTP.Hostname != "" {
		putStr("url.domain", log.Event.HTTP.Hostname)
	}
	if log.Event.HTTP.URL != "" {
		putStr(string(conventions.URLPathKey), log.Event.HTTP.URL)
	}
	if log.Event.HTTP.HTTPUserAgent != "" {
		putStr(string(conventions.UserAgentOriginalKey), log.Event.HTTP.HTTPUserAgent)
	}
	if log.Event.HTTP.HTTPContentType != "" {
		putStr("http.request.header.content-type", log.Event.HTTP.HTTPContentType)
	}
	if log.Event.HTTP.Cookie != "" {
		putStr("http.request.header.cookie", log.Event.HTTP.Cookie)
	}

	return nil
}
