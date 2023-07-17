// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// This file includes software developed at Datadog (https://www.datadoghq.com/)
// for the Datadog Agent (https://github.com/DataDog/datadog-agent)

// Package valid contains functions that validate hostnames
package valid // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/hostmetadata/valid"

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	validHostnameRfc1123 = regexp.MustCompile(`^(([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\-]*[a-zA-Z0-9])\.)*([A-Za-z0-9]|[A-Za-z0-9][A-Za-z0-9\-]*[A-Za-z0-9])$`)
	localhostIdentifiers = [...]string{
		"localhost",
		"localhost.localdomain",
		"localhost6.localdomain6",
		"ip6-localhost",
	}
)

// Hostname determines whether the passed string is a valid hostname.
// In case it's not, the returned error contains the details of the failure.
func Hostname(hostname string) error {
	const maxLength = 255

	switch {
	case hostname == "":
		return fmt.Errorf("hostname is empty")
	case isLocal(hostname):
		return fmt.Errorf("'%s' is a local hostname", hostname)
	case len(hostname) > maxLength:
		return fmt.Errorf("name exceeded the maximum length of %d characters", maxLength)
	case !validHostnameRfc1123.MatchString(hostname):
		return fmt.Errorf("'%s' is not RFC1123 compliant", hostname)
	}
	return nil
}

// check whether the name is in the list of local hostnames
func isLocal(name string) bool {
	name = strings.ToLower(name)
	for _, val := range localhostIdentifiers {
		if val == name {
			return true
		}
	}
	return false
}
