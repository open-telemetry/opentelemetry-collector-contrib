// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsefareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsefareceiver"

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// eniResolver resolves a MAC address to an ENI ID.
// Implementations should be safe for concurrent use. The HTTP client timeout
// (2s) provides the cancellation boundary; context propagation is not used
// because the ENI resolution is cached after the first call per device.
type eniResolver interface {
	GetENIID(macAddress string) (string, error)
}

type imdsENIResolver struct {
	client  *http.Client
	baseURL string // default: "http://169.254.169.254"
}

func newIMDSENIResolver() *imdsENIResolver {
	return &imdsENIResolver{
		client:  &http.Client{Timeout: 2 * time.Second},
		baseURL: "http://169.254.169.254",
	}
}

// getToken acquires an IMDSv2 session token.
func (r *imdsENIResolver) getToken() (string, error) {
	req, err := http.NewRequest(http.MethodPut, r.baseURL+"/latest/api/token", nil)
	if err != nil {
		return "", fmt.Errorf("failed to create token request: %w", err)
	}
	req.Header.Set("X-aws-ec2-metadata-token-ttl-seconds", "21600")
	resp, err := r.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("IMDS token request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("IMDS token request returned status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read IMDS token response: %w", err)
	}
	return strings.TrimSpace(string(body)), nil
}

// GetENIID resolves a MAC address to its ENI ID by querying the EC2 instance
// metadata service (IMDS). It acquires an IMDSv2 session token first, then
// looks up the interface-id for the given MAC.
func (r *imdsENIResolver) GetENIID(macAddress string) (string, error) {
	token, err := r.getToken()
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("%s/latest/meta-data/network/interfaces/macs/%s/interface-id", r.baseURL, macAddress)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create IMDS request: %w", err)
	}
	req.Header.Set("X-aws-ec2-metadata-token", token)

	resp, err := r.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("IMDS request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("IMDS returned status %d for MAC %s", resp.StatusCode, macAddress)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read IMDS response: %w", err)
	}
	return strings.TrimSpace(string(body)), nil
}
