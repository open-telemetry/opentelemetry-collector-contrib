// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resolver

import (
	"net"

	"github.com/asaskevich/govalidator"
)

// Flip the direction for logging
// if logKey is "hostname", return "ip" and vice versa
func Flip(logKey string) string {
	if logKey == LogKeyHostname {
		return LogKeyIP
	}
	return LogKeyHostname
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
