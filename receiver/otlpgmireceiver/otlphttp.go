// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otlpgmireceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otlpgmireceiver"

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otlpgmireceiver/internal/errors"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otlpgmireceiver/internal/logs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otlpgmireceiver/internal/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otlpgmireceiver/internal/profiles"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otlpgmireceiver/internal/trace"
)

// Pre-computed status with code=Internal to be used in case of a marshaling error.
var fallbackMsg = []byte(`{"code": 13, "message": "failed to marshal error message"}`)

const fallbackContentType = "application/json"

// UserInfo contains user information returned from license validation
type UserInfo struct {
	UserID string `json:"user_id"`
}

// LicenseValidationResponse represents the response from license validation service
type LicenseValidationResponse struct {
	Success bool     `json:"success"`
	Message string   `json:"message"`
	UserID  string   `json:"user_id"`
}

// LicenseCacheEntry contains cached license validation result
type LicenseCacheEntry struct {
	Valid    bool      `json:"valid"`
	UserInfo *UserInfo `json:"user_info,omitempty"`
	CachedAt time.Time `json:"cached_at"`
}

// LicenseValidator handles license validation with caching
type LicenseValidator struct {
	validationURL string
	cacheTimeout  time.Duration
	cache         map[string]*LicenseCacheEntry
	mutex         sync.RWMutex
	httpClient    *http.Client
}

// NewLicenseValidator creates a new license validator
func NewLicenseValidator(validationURL string, cacheTimeout int) *LicenseValidator {
	return &LicenseValidator{
		validationURL: validationURL,
		cacheTimeout:  time.Duration(cacheTimeout) * time.Second,
		cache:         make(map[string]*LicenseCacheEntry),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// ValidateLicense checks if a license key is valid and returns user info
func (lv *LicenseValidator) ValidateLicense(ctx context.Context, licenseKey string) (*UserInfo, bool) {
	// Check cache first
	lv.mutex.RLock()
	if entry, exists := lv.cache[licenseKey]; exists {
		if time.Since(entry.CachedAt) < lv.cacheTimeout {
			lv.mutex.RUnlock()
			return entry.UserInfo, entry.Valid
		}
	}
	lv.mutex.RUnlock()

	// Validate with external service
	userInfo, valid := lv.validateWithExternalService(ctx, licenseKey)

	// Update cache
	lv.mutex.Lock()
	if valid {
		lv.cache[licenseKey] = &LicenseCacheEntry{
			Valid:    valid,
			UserInfo: userInfo,
			CachedAt: time.Now(),
		}
	} else {
		delete(lv.cache, licenseKey)
	}
	lv.mutex.Unlock()

	return userInfo, valid
}

// validateWithExternalService makes HTTP request to validation URL
func (lv *LicenseValidator) validateWithExternalService(ctx context.Context, licenseKey string) (*UserInfo, bool) {
	// Prepare JSON request body
	requestBody := map[string]string{
		"license_key": licenseKey,
	}
	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, false
	}

	req, err := http.NewRequestWithContext(ctx, "POST", lv.validationURL, strings.NewReader(string(jsonBody)))
	if err != nil {
		return nil, false
	}
	
	// Set Content-Type header
	req.Header.Set("Content-Type", "application/json")

	resp, err := lv.httpClient.Do(req)
	if err != nil {
		return nil, false
	}
	defer resp.Body.Close()
	
	// Consider 200 OK as valid license
	if resp.StatusCode != http.StatusOK {
		return nil, false
	}

	// Parse validation response from response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, false
	}

	var validationResp LicenseValidationResponse
	if err := json.Unmarshal(body, &validationResp); err != nil {
		return nil, false
	}

	// Check if validation was successful
	if !validationResp.Success {
		return nil, false
	}

	// Create UserInfo from response
	userInfo := &UserInfo{
		UserID: validationResp.UserID,
	}

	return userInfo, true
}

// extractLicenseKeyFromPath extracts license key from URL path
// Expected format: /{licenseKey}/v1/traces, /{licenseKey}/v1/metrics, /{licenseKey}/v1/logs
func extractLicenseKeyFromPath(urlPath string) (string, error) {
	// Remove leading slash and split by slash
	parts := strings.Split(strings.TrimPrefix(urlPath, "/"), "/")
	
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid URL path format, expected /{licenseKey}/v1/{signal}")
	}

	licenseKey := parts[0]
	
	if licenseKey == "" {
		return "", fmt.Errorf("license key cannot be empty")
	}

	// Validate license key format (basic validation)
	if len(licenseKey) < 8 || len(licenseKey) > 64 {
		return "", fmt.Errorf("license key must be 8-64 characters long")
	}

	return licenseKey, nil
}

// addUserInfoToTracesData adds user information to trace resource attributes
func addUserInfoToTracesData(traces ptrace.Traces, userInfo *UserInfo) {
	if userInfo == nil {
		return
	}
	
	// Add user info to all resource spans
	resourceSpans := traces.ResourceSpans()
	for i := 0; i < resourceSpans.Len(); i++ {
		resource := resourceSpans.At(i).Resource()
		addUserInfoToResource(resource.Attributes(), userInfo)
	}
}

// addUserInfoToMetricsData adds user information to metrics resource attributes
func addUserInfoToMetricsData(metrics pmetric.Metrics, userInfo *UserInfo) {
	if userInfo == nil {
		return
	}
	
	// Add user info to all resource metrics
	resourceMetrics := metrics.ResourceMetrics()
	for i := 0; i < resourceMetrics.Len(); i++ {
		resource := resourceMetrics.At(i).Resource()
		addUserInfoToResource(resource.Attributes(), userInfo)
	}
}

// addUserInfoToLogsData adds user information to logs resource attributes
func addUserInfoToLogsData(logs plog.Logs, userInfo *UserInfo) {
	if userInfo == nil {
		return
	}
	
	// Add user info to all resource logs
	resourceLogs := logs.ResourceLogs()
	for i := 0; i < resourceLogs.Len(); i++ {
		resource := resourceLogs.At(i).Resource()
		addUserInfoToResource(resource.Attributes(), userInfo)
	}
}

// addUserInfoToProfilesData adds user information to profiles resource attributes
func addUserInfoToProfilesData(profiles pprofile.Profiles, userInfo *UserInfo) {
	if userInfo == nil {
		return
	}
	
	// Add user info to all resource profiles
	resourceProfiles := profiles.ResourceProfiles()
	for i := 0; i < resourceProfiles.Len(); i++ {
		resource := resourceProfiles.At(i).Resource()
		addUserInfoToResource(resource.Attributes(), userInfo)
	}
}

// addUserInfoToResource adds user information to resource attributes
func addUserInfoToResource(attrs pcommon.Map, userInfo *UserInfo) {
	if userInfo == nil {
		return
	}
	
	// Add user information as resource attributes
	if userInfo.UserID != "" {
		attrs.PutStr("gmi.otlp.user_id", userInfo.UserID)
	}
}

// writeLicenseError writes a license validation error response
func writeLicenseError(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	errorMsg := fmt.Sprintf(`{"error": "License validation failed", "message": "%s", "code": 403}`, message)
	w.Write([]byte(errorMsg))
}

func handleTraces(resp http.ResponseWriter, req *http.Request, tracesReceiver *trace.Receiver, licenseValidator *LicenseValidator) {
	var userInfo *UserInfo
	
	// Extract and validate license key if validator is provided
	if licenseValidator != nil {
		licenseKey, err := extractLicenseKeyFromPath(req.URL.Path)
		if err != nil {
			writeLicenseError(resp, fmt.Sprintf("Invalid license key in URL path: %v", err))
			return
		}

		var valid bool
		userInfo, valid = licenseValidator.ValidateLicense(req.Context(), licenseKey)
		if !valid {
			writeLicenseError(resp, "License validation failed")
			return
		}
	}

	enc, ok := readContentType(resp, req)
	if !ok {
		return
	}

	body, ok := readAndCloseBody(resp, req, enc)
	if !ok {
		return
	}

	otlpReq, err := enc.unmarshalTracesRequest(body)
	if err != nil {
		writeError(resp, enc, err, http.StatusBadRequest)
		return
	}

	// Add user information to traces if available
	if userInfo != nil {
		addUserInfoToTracesData(otlpReq.Traces(), userInfo)
	}

	otlpResp, err := tracesReceiver.Export(req.Context(), otlpReq)
	if err != nil {
		writeError(resp, enc, err, http.StatusInternalServerError)
		return
	}

	msg, err := enc.marshalTracesResponse(otlpResp)
	if err != nil {
		writeError(resp, enc, err, http.StatusInternalServerError)
		return
	}
	writeResponse(resp, enc.contentType(), http.StatusOK, msg)
}

func handleMetrics(resp http.ResponseWriter, req *http.Request, metricsReceiver *metrics.Receiver, licenseValidator *LicenseValidator) {
	var userInfo *UserInfo
	
	// Extract and validate license key if validator is provided
	if licenseValidator != nil {
		licenseKey, err := extractLicenseKeyFromPath(req.URL.Path)
		if err != nil {
			writeLicenseError(resp, fmt.Sprintf("Invalid license key in URL path: %v", err))
			return
		}

		var valid bool
		userInfo, valid = licenseValidator.ValidateLicense(req.Context(), licenseKey)
		if !valid {
			writeLicenseError(resp, "License validation failed")
			return
		}
	}

	enc, ok := readContentType(resp, req)
	if !ok {
		return
	}

	body, ok := readAndCloseBody(resp, req, enc)
	if !ok {
		return
	}

	otlpReq, err := enc.unmarshalMetricsRequest(body)
	if err != nil {
		writeError(resp, enc, err, http.StatusBadRequest)
		return
	}

	// Add user information to metrics if available
	if userInfo != nil {
		addUserInfoToMetricsData(otlpReq.Metrics(), userInfo)
	}

	otlpResp, err := metricsReceiver.Export(req.Context(), otlpReq)
	if err != nil {
		writeError(resp, enc, err, http.StatusInternalServerError)
		return
	}

	msg, err := enc.marshalMetricsResponse(otlpResp)
	if err != nil {
		writeError(resp, enc, err, http.StatusInternalServerError)
		return
	}
	writeResponse(resp, enc.contentType(), http.StatusOK, msg)
}

func handleLogs(resp http.ResponseWriter, req *http.Request, logsReceiver *logs.Receiver, licenseValidator *LicenseValidator) {
	var userInfo *UserInfo
	
	// Extract and validate license key if validator is provided
	if licenseValidator != nil {
		licenseKey, err := extractLicenseKeyFromPath(req.URL.Path)
		if err != nil {
			writeLicenseError(resp, fmt.Sprintf("Invalid license key in URL path: %v", err))
			return
		}

		var valid bool
		userInfo, valid = licenseValidator.ValidateLicense(req.Context(), licenseKey)
		if !valid {
			writeLicenseError(resp, "License validation failed")
			return
		}
	}

	enc, ok := readContentType(resp, req)
	if !ok {
		return
	}

	body, ok := readAndCloseBody(resp, req, enc)
	if !ok {
		return
	}

	otlpReq, err := enc.unmarshalLogsRequest(body)
	if err != nil {
		writeError(resp, enc, err, http.StatusBadRequest)
		return
	}

	// Add user information to logs if available
	if userInfo != nil {
		addUserInfoToLogsData(otlpReq.Logs(), userInfo)
	}

	otlpResp, err := logsReceiver.Export(req.Context(), otlpReq)
	if err != nil {
		writeError(resp, enc, err, http.StatusInternalServerError)
		return
	}

	msg, err := enc.marshalLogsResponse(otlpResp)
	if err != nil {
		writeError(resp, enc, err, http.StatusInternalServerError)
		return
	}
	writeResponse(resp, enc.contentType(), http.StatusOK, msg)
}

func handleProfiles(resp http.ResponseWriter, req *http.Request, profilesReceiver *profiles.Receiver, licenseValidator *LicenseValidator) {
	var userInfo *UserInfo
	
	// Extract and validate license key if validator is provided
	if licenseValidator != nil {
		licenseKey, err := extractLicenseKeyFromPath(req.URL.Path)
		if err != nil {
			writeLicenseError(resp, fmt.Sprintf("Invalid license key in URL path: %v", err))
			return
		}

		var valid bool
		userInfo, valid = licenseValidator.ValidateLicense(req.Context(), licenseKey)
		if !valid {
			writeLicenseError(resp, "License validation failed")
			return
		}
	}

	enc, ok := readContentType(resp, req)
	if !ok {
		return
	}

	body, ok := readAndCloseBody(resp, req, enc)
	if !ok {
		return
	}

	otlpReq, err := enc.unmarshalProfilesRequest(body)
	if err != nil {
		writeError(resp, enc, err, http.StatusBadRequest)
		return
	}

	// Add user information to profiles if available
	if userInfo != nil {
		addUserInfoToProfilesData(otlpReq.Profiles(), userInfo)
	}

	otlpResp, err := profilesReceiver.Export(req.Context(), otlpReq)
	if err != nil {
		writeError(resp, enc, err, http.StatusInternalServerError)
		return
	}

	msg, err := enc.marshalProfilesResponse(otlpResp)
	if err != nil {
		writeError(resp, enc, err, http.StatusInternalServerError)
		return
	}
	writeResponse(resp, enc.contentType(), http.StatusOK, msg)
}

func readContentType(resp http.ResponseWriter, req *http.Request) (encoder, bool) {
	if req.Method != http.MethodPost {
		handleUnmatchedMethod(resp)
		return nil, false
	}

	switch getMimeTypeFromContentType(req.Header.Get("Content-Type")) {
	case pbContentType:
		return pbEncoder, true
	case jsonContentType:
		return jsEncoder, true
	default:
		handleUnmatchedContentType(resp)
		return nil, false
	}
}

func readAndCloseBody(resp http.ResponseWriter, req *http.Request, enc encoder) ([]byte, bool) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		writeError(resp, enc, err, http.StatusBadRequest)
		return nil, false
	}
	if err = req.Body.Close(); err != nil {
		writeError(resp, enc, err, http.StatusBadRequest)
		return nil, false
	}
	return body, true
}

// writeError encodes the HTTP error inside a rpc.Status message as required by the OTLP protocol.
func writeError(w http.ResponseWriter, encoder encoder, err error, statusCode int) {
	s, ok := status.FromError(err)
	if ok {
		statusCode = errors.GetHTTPStatusCodeFromStatus(s)
	} else {
		s = newStatusFromMsgAndHTTPCode(err.Error(), statusCode)
	}
	writeStatusResponse(w, encoder, statusCode, s)
}

// errorHandler encodes the HTTP error message inside a rpc.Status message as required
// by the OTLP protocol.
func errorHandler(w http.ResponseWriter, r *http.Request, errMsg string, statusCode int) {
	s := newStatusFromMsgAndHTTPCode(errMsg, statusCode)
	switch getMimeTypeFromContentType(r.Header.Get("Content-Type")) {
	case pbContentType:
		writeStatusResponse(w, pbEncoder, statusCode, s)
		return
	case jsonContentType:
		writeStatusResponse(w, jsEncoder, statusCode, s)
		return
	}
	writeResponse(w, fallbackContentType, http.StatusInternalServerError, fallbackMsg)
}

func writeStatusResponse(w http.ResponseWriter, enc encoder, statusCode int, st *status.Status) {
	// https://github.com/open-telemetry/opentelemetry-proto/blob/main/docs/specification.md#otlphttp-throttling
	if statusCode == http.StatusTooManyRequests || statusCode == http.StatusServiceUnavailable {
		retryInfo := getRetryInfo(st)
		// Check if server returned throttling information.
		if retryInfo != nil {
			// We are throttled. Wait before retrying as requested by the server.
			// The value of Retry-After field can be either an HTTP-date or a number of
			// seconds to delay after the response is received. See https://datatracker.ietf.org/doc/html/rfc7231#section-7.1.3
			//
			// Retry-After = HTTP-date / delay-seconds
			//
			// Use delay-seconds since is easier to format as well as does not require clock synchronization.
			// For simplicity, use a default retry delay of 1 second
			w.Header().Set("Retry-After", "1")
		}
	}
	msg, err := enc.marshalStatus(st.Proto())
	if err != nil {
		writeResponse(w, fallbackContentType, http.StatusInternalServerError, fallbackMsg)
		return
	}

	writeResponse(w, enc.contentType(), statusCode, msg)
}

func writeResponse(w http.ResponseWriter, contentType string, statusCode int, msg []byte) {
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(statusCode)
	// Nothing we can do with the error if we cannot write to the response.
	_, _ = w.Write(msg)
}

func getMimeTypeFromContentType(contentType string) string {
	mediatype, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return ""
	}
	return mediatype
}

func handleUnmatchedMethod(resp http.ResponseWriter) {
	hst := http.StatusMethodNotAllowed
	writeResponse(resp, "text/plain", hst, []byte(fmt.Sprintf("%v method not allowed, supported: [POST]", hst)))
}

func handleUnmatchedContentType(resp http.ResponseWriter) {
	hst := http.StatusUnsupportedMediaType
	writeResponse(resp, "text/plain", hst, []byte(fmt.Sprintf("%v unsupported media type, supported: [%s, %s]", hst, jsonContentType, pbContentType)))
}

// newStatusFromMsgAndHTTPCode creates a new status from message and HTTP code
func newStatusFromMsgAndHTTPCode(msg string, httpCode int) *status.Status {
	// Map HTTP status codes to gRPC status codes
	var code codes.Code
	switch httpCode {
	case http.StatusOK:
		code = codes.OK
	case http.StatusBadRequest:
		code = codes.InvalidArgument
	case http.StatusUnauthorized:
		code = codes.Unauthenticated
	case http.StatusForbidden:
		code = codes.PermissionDenied
	case http.StatusNotFound:
		code = codes.NotFound
	case http.StatusTooManyRequests:
		code = codes.ResourceExhausted
	case http.StatusInternalServerError:
		code = codes.Internal
	case http.StatusServiceUnavailable:
		code = codes.Unavailable
	default:
		code = codes.Unknown
	}
	return status.New(code, msg)
}

// getRetryInfo extracts retry information from a gRPC status
func getRetryInfo(st *status.Status) interface{} {
	// For simplicity, return nil as retry info is not critical for basic functionality
	// In a full implementation, this would extract retry information from the status details
	return nil
}
