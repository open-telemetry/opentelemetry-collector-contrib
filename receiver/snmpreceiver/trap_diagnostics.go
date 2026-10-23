// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package snmpreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snmpreceiver"

import (
	"context"
	"errors"
	"net"
	"time"

	"go.uber.org/zap"
)

var (
	errTrapCommunityNotAllowed       = errors.New("SNMP notification community is not allowed")
	errTrapUserNotAllowed            = errors.New("SNMPv3 notification user is not allowed")
	errTrapAuthenticationFailed      = errors.New("SNMPv3 notification is not authentic")
	errTrapAuthenticationParameters  = errors.New("invalid SNMPv3 authentication parameters")
	errTrapSecurityLevelNotAllowed   = errors.New("SNMPv3 notification security level is not allowed")
	errTrapSecurityLevelBelowMinimum = errors.New("SNMPv3 notification security level is below the configured minimum")
	errTrapUSMNotConfigured          = errors.New("SNMPv3 USM is not configured")
	errTrapTimeWindow                = errors.New("SNMPv3 notification is outside the USM time window")
	errTrapEngineLimit               = errors.New("SNMPv3 notification engine limit reached")
	errTrapVersionNotEnabled         = errors.New("SNMP notification version is not enabled")
	errTrapV3InformUnsupported       = errors.New("SNMPv3 informs are not supported")
)

// rejectTrap is called only by the receive goroutine. Sample diagnostics across
// all peers and reasons without keeping per-peer state or sampling telemetry.
func (r *trapReceiver) rejectTrap(ctx context.Context, peer net.Addr, count int, fallbackReason string, err error) {
	r.obs.EndLogsOp(ctx, trapDataFormat, count, err)
	if !r.logger.Core().Enabled(zap.DebugLevel) {
		return
	}
	now := time.Now()
	if now.Before(r.nextRejectionLog) {
		return
	}
	r.nextRejectionLog = now.Add(time.Second)
	if entry := r.logger.Check(zap.DebugLevel, "Rejected SNMP notification"); entry != nil {
		address := "unknown"
		if peer != nil {
			address = peer.String()
		}
		entry.Write(zap.String("peer", address), zap.String("reason", trapRejectionReason(err, fallbackReason)))
	}
}

// Only fixed reason codes are logged. Decoder errors can contain packet values,
// and dependency errors must retain the existing raw-BER sanitization.
func trapRejectionReason(err error, fallback string) string {
	switch {
	case errors.Is(err, errTrapCommunityNotAllowed):
		return "community_not_allowed"
	case errors.Is(err, errTrapUserNotAllowed):
		return "user_not_allowed"
	case errors.Is(err, errTrapAuthenticationFailed):
		return "authentication_failed"
	case errors.Is(err, errTrapAuthenticationParameters):
		return "invalid_authentication_parameters"
	case errors.Is(err, errTrapSecurityLevelNotAllowed), errors.Is(err, errTrapSecurityLevelBelowMinimum):
		return "security_level_not_allowed"
	case errors.Is(err, errTrapUSMNotConfigured):
		return "usm_not_configured"
	case errors.Is(err, errTrapTimeWindow):
		return "outside_time_window"
	case errors.Is(err, errTrapEngineLimit):
		return "engine_limit_reached"
	case errors.Is(err, errTrapVersionNotEnabled):
		return "version_not_enabled"
	case errors.Is(err, errTrapV3InformUnsupported):
		return "v3_informs_unsupported"
	default:
		return fallback
	}
}
