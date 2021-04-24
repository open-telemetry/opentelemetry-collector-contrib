// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package awsprometheusremotewriteexporter provides a Prometheus Remote Write Exporter with AWS Sigv4 authentication
package awsprometheusremotewriteexporter

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/session"
	v4 "github.com/aws/aws-sdk-go/aws/signer/v4"
)

// signingRoundTripper is a Custom RoundTripper that performs AWS Sig V4
type signingRoundTripper struct {
	transport http.RoundTripper
	signer    *v4.Signer
	region    string
	service   string
}

// RoundTrip signs each outgoing request
func (si *signingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	reqBody, err := req.GetBody()
	if err != nil {
		return nil, err
	}

	// Get the body
	content, err := ioutil.ReadAll(reqBody)
	reqBody.Close()
	if err != nil {
		return nil, err
	}

	body := bytes.NewReader(content)

	// Add the sdk and system information to the User-Agent header of the request
	addToUserAgent(req, formatHeaderValues(aws.SDKName, aws.SDKVersion, runtime.Version(), runtime.GOOS, runtime.GOARCH))

	v := os.Getenv("AWS_EXECUTION_ENV")
	if len(v) != 0 {
		addToUserAgent(req, formatHeaderValues("exec_env", v))
	}

	// Clone request to ensure thread safety
	req2 := cloneRequest(req)

	// Sign the request
	_, err = si.signer.Sign(req2, body, si.service, si.region, time.Now())
	if err != nil {
		return nil, err
	}

	// Send the request to Prometheus Remote Write Backend
	resp, err := si.transport.RoundTrip(req2)
	if err != nil {
		return nil, err
	}

	return resp, err
}

func newSigningRoundTripper(auth AuthConfig, next http.RoundTripper) (http.RoundTripper, error) {

	creds := getCredsFromConfig(auth)
	return createSigningRoundTripperWithCredentials(auth, creds, next)
}

func getCredsFromConfig(auth AuthConfig) *credentials.Credentials {

	// Session Must ensure the Session is valid
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		Config: aws.Config{Region: aws.String(auth.Region)},
	}))

	var creds *credentials.Credentials
	if auth.RoleArn != "" {
		// Get credentials from an assumeRole API call
		creds = stscreds.NewCredentials(sess, auth.RoleArn, func(p *stscreds.AssumeRoleProvider) {
			p.RoleSessionName = "aws-otel-collector-" + strconv.FormatInt(time.Now().Unix(), 10)
		})
	} else {
		// Get Credentials, either from ./aws or from environmental variables
		creds = sess.Config.Credentials
	}
	return creds
}

func createSigningRoundTripperWithCredentials(auth AuthConfig, creds *credentials.Credentials, next http.RoundTripper) (http.RoundTripper, error) {
	if !isValidAuth(auth) {
		return next, nil
	}

	if creds == nil {
		return nil, errors.New("no AWS credentials exist")
	}

	signer := v4.NewSigner(creds)

	rt := signingRoundTripper{
		transport: next,
		signer:    signer,
		region:    auth.Region,
		service:   auth.Service,
	}

	// return a RoundTripper
	return &rt, nil
}

func isValidAuth(params AuthConfig) bool {
	return params.Region != "" && params.Service != ""
}

func cloneRequest(r *http.Request) *http.Request {
	// shallow copy of the struct
	r2 := new(http.Request)
	*r2 = *r
	// deep copy of the Header
	r2.Header = make(http.Header, len(r.Header))
	for k, s := range r.Header {
		r2.Header[k] = append([]string(nil), s...)
	}
	return r2
}

// formatHeaderValues will format the provided name and version to the name/version format.
// If the extra parameters are provided they will be added as metadata to the
// name/version pair resulting in the following format.
// "name/version (extra0; extra1; ...)"
func formatHeaderValues(name, version string, extra ...string) string {
	ua := fmt.Sprintf("%s/%s", name, version)
	if len(extra) > 0 {
		ua = fmt.Sprintf("%s (%s)", ua, strings.Join(extra, "; "))
	}
	// AddToUserAgent(req, ua)
	return ua
}

// addToUserAgent adds the string to the end of the request's current User-Agent.
func addToUserAgent(req *http.Request, s string) {
	curUA := req.Header.Get("User-Agent")
	if len(curUA) > 0 {
		s = curUA + " " + s
	}
	req.Header.Set("User-Agent", s)
}
