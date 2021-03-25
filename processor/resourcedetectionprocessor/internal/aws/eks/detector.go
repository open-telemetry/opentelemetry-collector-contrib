// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package eks

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/conventions"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

const (
	// TypeStr is type of detector.
	TypeStr = "eks"

	/* #nosec */
	k8sTokenPath      = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	k8sCertPath       = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
	authConfigmapPath = "https://kubernetes.default.svc/api/v1/namespaces/kube-system/configmaps/aws-auth"
	cwConfigmapPath   = "https://kubernetes.default.svc/api/v1/namespaces/amazon-cloudwatch/configmaps/cluster-info"
)

// detectorUtils is used for testing the resourceDetector by abstracting functions that rely on external systems.
type detectorUtils interface {
	// fileExists returns true if the file exists, otherwise false.
	fileExists(filename string) bool
	// fetchString executes an HTTP request with a given HTTP Method and URL string returning
	// the content body or an error.
	fetchString(ctx context.Context, httpMethod string, URL string) (string, error)
}

// This struct will implement the detectorUtils interface
type eksDetectorUtils struct{}

// This struct will help unmarshal clustername from JSON response
type data struct {
	ClusterName string `json:"cluster.name"`
}

var _ internal.Detector = (*Detector)(nil)

// Detector for EKS
type Detector struct {
	utils detectorUtils
}

// Compile time assertion that eksDetectorUtils implements the detectorUtils interface.
var _ detectorUtils = (*eksDetectorUtils)(nil)

// NewDetector returns a resource detector that will detect AWS EKS resources.
func NewDetector(_ component.ProcessorCreateParams, _ internal.DetectorConfig) (internal.Detector, error) {
	return &Detector{utils: &eksDetectorUtils{}}, nil
}

// Detect returns a Resource describing the Amazon EKS environment being run in.
func (detector *Detector) Detect(ctx context.Context) (pdata.Resource, error) {
	res := pdata.NewResource()
	isEks, err := isEKS(ctx, detector.utils)
	if err != nil {
		return res, err
	}

	// Return empty resource object if not running in EKS
	if !isEks {
		return res, nil
	}

	attr := res.Attributes()
	attr.InsertString(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAWS)
	attr.InsertString(conventions.AttributeCloudInfrastructureService, conventions.AttributeCloudProviderAWSEKS)

	// Get clusterName and append to attributes
	clusterName, err := getClusterName(ctx, detector.utils)
	if err != nil {
		return res, err
	}
	if clusterName != "" {
		attr.InsertString(conventions.AttributeK8sCluster, clusterName)
	}

	return res, nil
}

// isEKS checks if the current environment is running in EKS.
func isEKS(ctx context.Context, utils detectorUtils) (bool, error) {
	if !isK8s(utils) {
		return false, nil
	}

	// Make HTTP GET request
	awsAuth, err := utils.fetchString(ctx, http.MethodGet, authConfigmapPath)
	if err != nil {
		return false, fmt.Errorf("isEks() error retrieving auth configmap: %w", err)
	}

	return awsAuth != "", nil
}

// isK8s checks if the current environment is running in a Kubernetes environment
func isK8s(utils detectorUtils) bool {
	return utils.fileExists(k8sTokenPath) && utils.fileExists(k8sCertPath)
}

// fileExists returns true if the file exists, otherwise false.
func (eksUtils eksDetectorUtils) fileExists(filename string) bool {
	info, err := os.Stat(filename)
	return err == nil && !info.IsDir()
}

// fetchString executes an HTTP request with a given HTTP Method and URL string returning
// the content body or an error.
func (eksUtils eksDetectorUtils) fetchString(ctx context.Context, httpMethod string, URL string) (string, error) {
	request, err := http.NewRequestWithContext(ctx, httpMethod, URL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create new HTTP request with method=%s, URL=%s: %w", httpMethod, URL, err)
	}

	// Set HTTP request header with authentication credentials
	k8sToken, err := getK8sToken()
	if err != nil {
		return "", err
	}
	request.Header.Set("Authorization", "Bearer "+k8sToken)

	// Get certificate
	caCert, err := ioutil.ReadFile(k8sCertPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file with path %s", k8sCertPath)
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	// Set HTTP request timeout and add certificate
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: caCertPool,
			},
		},
	}

	response, err := client.Do(request)
	if err != nil || response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to execute HTTP request with method=%s, URL=%s, Status Code=%d: %w", httpMethod, URL, response.StatusCode, err)
	}

	// Retrieve response body from HTTP request
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response from HTTP request with method=%s, URL=%s: %w", httpMethod, URL, err)
	}

	return string(body), nil
}

// getK8sToken retrieves the kubernetes credential information.
func getK8sToken() (string, error) {
	content, err := ioutil.ReadFile(k8sTokenPath)
	if err != nil {
		return "", fmt.Errorf("getK8sToken() error: cannot read file with path %s", k8sTokenPath)
	}

	return string(content), nil
}

// getClusterName retrieves the clusterName resource attribute
func getClusterName(ctx context.Context, utils detectorUtils) (string, error) {
	resp, err := utils.fetchString(ctx, "GET", cwConfigmapPath)
	if err != nil {
		return "", fmt.Errorf("getClusterName() error: %w", err)
	}

	// parse JSON object returned from HTTP request
	var respmap map[string]json.RawMessage
	err = json.Unmarshal([]byte(resp), &respmap)
	if err != nil {
		return "", fmt.Errorf("getClusterName() error: cannot parse JSON: %w", err)
	}
	var d data
	err = json.Unmarshal(respmap["data"], &d)
	if err != nil {
		return "", fmt.Errorf("getClusterName() error: cannot parse JSON: %w", err)
	}

	clusterName := d.ClusterName

	return clusterName, nil
}
