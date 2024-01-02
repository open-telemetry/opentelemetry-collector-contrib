// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sigv4authextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/sigv4authextension"

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	sigv4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"go.uber.org/zap"
)

var errNilRequest = errors.New("sigv4: unable to sign nil *http.Request")

// signingRoundTripper is a custom RoundTripper that performs AWS Sigv4.
type signingRoundTripper struct {
	transport     http.RoundTripper
	signer        *sigv4.Signer
	region        string
	service       string
	credsProvider *aws.CredentialsProvider
	awsSDKInfo    string
	logger        *zap.Logger
}

// RoundTrip() executes a single HTTP transaction and returns an HTTP response, signing
// the request with Sigv4.
func (si *signingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if req == nil {
		si.logger.Warn("nil *http.Request encountered")
		return nil, errNilRequest
	}

	req2, err := si.signRequest(req)
	if err != nil {
		si.logger.Debug("error signing request", zap.Error(err))
		return nil, err
	}

	// Send the request
	return si.transport.RoundTrip(req2)
}

func (si *signingRoundTripper) signRequest(req *http.Request) (*http.Request, error) {
	payloadHash, err := hashPayload(req)
	if err != nil {
		return nil, fmt.Errorf("unable to hash request body: %w", err)
	}

	// Clone request to ensure thread safety.
	req2 := cloneRequest(req)

	// Add the runtime information to the User-Agent header of the request
	ua := req2.Header.Get("User-Agent")
	if len(ua) > 0 {
		ua = ua + " " + si.awsSDKInfo
	} else {
		ua = si.awsSDKInfo
	}
	req2.Header.Set("User-Agent", ua)

	// Use user provided service/region if specified, use inferred service/region if not, then sign the request
	service, region := si.inferServiceAndRegion(req2)
	if si.credsProvider == nil {
		return nil, fmt.Errorf("a credentials provider is not set")
	}
	creds, err := (*si.credsProvider).Retrieve(req2.Context())
	if err != nil {
		return nil, fmt.Errorf("error retrieving credentials: %w", err)
	}

	err = si.signer.SignHTTP(req.Context(), creds, req2, payloadHash, service, region, time.Now())
	if err != nil {
		return nil, fmt.Errorf("error signing the request: %w", err)
	}

	return req2, nil
}

// hashPayload creates a SHA256 hash of the request body
func hashPayload(req *http.Request) (string, error) {
	if req.GetBody == nil {
		// hash of an empty payload to use if there is no request body
		return "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", nil
	}

	reqBody, err := req.GetBody()
	if err != nil {
		return "", err
	}

	// Hash the request body
	h := sha256.New()
	_, err = io.Copy(h, reqBody)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), reqBody.Close()
}

// inferServiceAndRegion attempts to infer a service
// and a region from an http.request, and returns either an empty
// string for both or a valid value for both.
func (si *signingRoundTripper) inferServiceAndRegion(r *http.Request) (service string, region string) {
	service = si.service
	region = si.region

	h := r.Host
	if strings.HasPrefix(h, "aps-workspaces") {
		if service == "" {
			service = "aps"
		}
		rest := h[strings.Index(h, ".")+1:]
		if region == "" {
			region = rest[0:strings.Index(rest, ".")]
		}
	} else if strings.HasPrefix(h, "search-") {
		if service == "" {
			service = "es"
		}
		rest := h[strings.Index(h, ".")+1:]
		if region == "" {
			region = rest[0:strings.Index(rest, ".")]
		}
	}

	if service == "" || region == "" {
		si.logger.Warn("Unable to infer region and/or service from the URL. Please provide values for region and/or service in the collector configuration.")
	}
	return service, region
}

// cloneRequest() is a helper function that makes a shallow copy of the request and a
// deep copy of the header, for thread safety purposes.
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
