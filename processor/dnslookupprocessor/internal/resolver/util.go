// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resolver

import (
	"net"
	"strings"

	"github.com/asaskevich/govalidator"
)

// Flip the direction for logging
func Flip(logKey string) string {
	switch logKey {
	case LogKeyHostname:
		return LogKeyIP
	case LogKeyIP:
		return LogKeyHostname
	default:
		return LogKeyHostname
	}
}

func ParseIP(ip string) (string, error) {
	netIP := net.ParseIP(ip)
	if netIP == nil || netIP.IsUnspecified() {
		return "", ErrInvalidIP
	}

	return ip, nil
}

func ParseHostname(hostname string) (string, error) {
	if isValid := govalidator.IsDNSName(hostname); !isValid {
		return "", ErrInvalidHostname
	}

	return hostname, nil
}

// RemoveTrailingDot removes a trailing dot from a hostname if present
// Note: LookupAddr results typically have a trailing dot which can be removed
func RemoveTrailingDot(hostname string) string {
	if len(hostname) > 0 && hostname[len(hostname)-1] == '.' {
		return hostname[:len(hostname)-1]
	}
	return hostname
}

func NormalizeHostname(hostname string) string {
	hostname = RemoveTrailingDot(hostname)
	return strings.ToLower(hostname)
}
