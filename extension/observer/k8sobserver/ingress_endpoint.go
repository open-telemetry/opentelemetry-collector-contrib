// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/k8sobserver"

import (
	"fmt"
	"net/url"
	"strings"

	v1 "k8s.io/api/networking/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

// convertIngressToEndpoints converts a ingress instance into a slice of endpoints. The endpoints
// include an endpoint for each path that is mapped to an ingress.
func convertIngressToEndpoints(idNamespace string, ingress *v1.Ingress) []observer.Endpoint {
	endpoints := []observer.Endpoint{}

	// Loop through every ingress rule to get every defined path.
	for _, rule := range ingress.Spec.Rules {
		scheme := getScheme(rule.Host, getTLSHosts(ingress))

		if rule.HTTP != nil {
			// Create endpoint for each ingress rule.
			for _, path := range rule.HTTP.Paths {
				endpointID := observer.EndpointID(fmt.Sprintf("%s/%s/%s%s", idNamespace, ingress.UID, rule.Host, path.Path))
				endpoints = append(endpoints, observer.Endpoint{
					ID: endpointID,
					Target: (&url.URL{
						Scheme: scheme,
						Host:   rule.Host,
						Path:   path.Path,
					}).String(),
					Details: &observer.K8sIngress{
						Name:        ingress.Name,
						UID:         string(endpointID),
						Labels:      ingress.Labels,
						Annotations: ingress.Annotations,
						Namespace:   ingress.Namespace,
						Scheme:      scheme,
						Host:        rule.Host,
						Path:        path.Path,
					},
				})
			}
		}
	}

	return endpoints
}

// getTLSHosts return a list of tls hosts for an ingress resource.
func getTLSHosts(i *v1.Ingress) []string {
	var hosts []string

	for _, tls := range i.Spec.TLS {
		hosts = append(hosts, tls.Hosts...)
	}

	return hosts
}

// matchesHostPattern returns true if the host matches the host pattern or wildcard pattern.
func matchesHostPattern(pattern string, host string) bool {
	// if host match the pattern (host pattern).
	if pattern == host {
		return true
	}

	// if string does not contains any dot, don't do the next part as it's for wildcard pattern.
	if !strings.Contains(host, ".") {
		return false
	}

	patternParts := strings.Split(pattern, ".")
	hostParts := strings.Split(host, ".")

	// If the first part of the pattern is not a wildcard pattern.
	if patternParts[0] != "*" {
		return false
	}

	// If host and pattern without wildcard part does not match.
	if strings.Join(patternParts[1:], ".") != strings.Join(hostParts[1:], ".") {
		return false
	}

	return true
}

// getScheme return the scheme of an ingress host based on tls configuration.
func getScheme(host string, tlsHosts []string) string {
	for _, pattern := range tlsHosts {
		if matchesHostPattern(pattern, host) {
			return "https"
		}
	}

	return "http"
}
