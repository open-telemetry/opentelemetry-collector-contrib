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

package opsrampotlpexporter // import "go.opentelemetry.io/collector/exporter/otlpexporter"

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"runtime"
	"time"

	"go.opentelemetry.io/collector/config"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
)

type exporter struct {
	// Input configuration.
	config *Config

	// gRPC clients and connection.
	traceExporter  ptraceotlp.Client
	metricExporter pmetricotlp.Client
	logExporter    plogotlp.Client
	clientConn     *grpc.ClientConn
	metadata       metadata.MD
	callOptions    []grpc.CallOption

	settings component.TelemetrySettings

	// Default user-agent header.
	userAgent   string
	accessToken string
}

// Crete new exporter and start it. The exporter will begin connecting but
// this function may return before the connection is established.
func newExporter(cfg config.Exporter, set component.ExporterCreateSettings) (*exporter, error) {
	oCfg := cfg.(*Config)

	accessToken, err := getAuthToken(oCfg.Security)
	if err != nil {
		return nil, fmt.Errorf("access token isn't available: %w", err)
	}

	if oCfg.Endpoint == "" {
		return nil, errors.New("OTLP exporter config requires an Endpoint")
	}

	userAgent := fmt.Sprintf("%s/%s (%s/%s)",
		set.BuildInfo.Description, set.BuildInfo.Version, runtime.GOOS, runtime.GOARCH)

	return &exporter{config: oCfg, settings: set.TelemetrySettings, userAgent: userAgent, accessToken: accessToken}, nil
}

type Creds struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	GrantType    string `json:"grant_type"`
}

type Person struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

func getAuthToken(cfg SecuritySettings) (string, error) {

	client := &http.Client{}
	data := url.Values{}
	data.Set("client_id", cfg.ClientId)
	data.Set("client_secret", cfg.ClientSecret)
	data.Set("grant_type", grantType)

	request, err := http.NewRequest("POST", cfg.OAuthServiceURL, bytes.NewBufferString(data.Encode()))
	if err != nil {
		return "", err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(request)

	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	jsonResp, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var credentials Credentials
	if err := json.Unmarshal(jsonResp, &credentials); err != nil {
		return "", err
	}
	return credentials.AccessToken, nil

}

// start actually creates the gRPC connection. The client construction is deferred till this point as this
// is the only place we get hold of Extensions which are required to construct auth round tripper.
func (e *exporter) start(ctx context.Context, host component.Host) (err error) {
	dialOpts, err := e.config.GRPCClientSettings.ToDialOptions(host, e.settings)
	if err != nil {
		return err
	}
	dialOpts = append(dialOpts, grpc.WithUserAgent(e.userAgent))

	if e.clientConn, err = grpc.DialContext(ctx, e.config.GRPCClientSettings.SanitizedEndpoint(), dialOpts...); err != nil {
		return err
	}

	e.traceExporter = ptraceotlp.NewClient(e.clientConn)
	e.metricExporter = pmetricotlp.NewClient(e.clientConn)
	e.logExporter = plogotlp.NewClient(e.clientConn)
	e.metadata = metadata.New(e.config.GRPCClientSettings.Headers)
	e.metadata.Set("Authorization", fmt.Sprintf("Bearer %s", e.accessToken))
	e.callOptions = []grpc.CallOption{
		grpc.WaitForReady(e.config.GRPCClientSettings.WaitForReady),
	}

	return
}

func (e *exporter) shutdown(context.Context) error {
	return e.clientConn.Close()
}

func (e *exporter) pushTraces(ctx context.Context, td ptrace.Traces) error {
	req := ptraceotlp.NewRequestFromTraces(td)
	_, err := e.traceExporter.Export(e.enhanceContext(ctx), req, e.callOptions...)
	return processError(err)
}

func (e *exporter) pushMetrics(ctx context.Context, md pmetric.Metrics) error {
	req := pmetricotlp.NewRequestFromMetrics(md)
	_, err := e.metricExporter.Export(e.enhanceContext(ctx), req, e.callOptions...)
	return processError(err)
}

func (e *exporter) pushLogs(ctx context.Context, ld plog.Logs) error {
	if e.config.Masking != nil {
		e.applyMasking(ld)
	}
	if e.config.ExpirationSkip != 0 {
		e.skipExpired(ld)
	}

	req := plogotlp.NewRequestFromLogs(ld)

	_, err := e.logExporter.Export(e.enhanceContext(ctx), req, e.callOptions...)

	// trying to get new access token in case of expiration
	if err != nil {
		st := status.Convert(err)
		if st.Code() == codes.Unauthenticated {
			if err := e.updateExpiredToken(); err != nil {
				return fmt.Errorf("couldn't retreive new token instead of expired: %w", err)
			}

			_, err = e.logExporter.Export(e.enhanceContext(ctx), req, e.callOptions...)
			if err != nil {
				return err
			}
		}
		return processError(err)
	}
	return nil
}

func (e *exporter) updateExpiredToken() error {
	accessToken, err := getAuthToken(e.config.Security)
	if err != nil {
		return err
	}
	e.metadata.Set("bearer", accessToken)
	return nil
}

func (e *exporter) enhanceContext(ctx context.Context) context.Context {
	if e.metadata.Len() > 0 {
		return metadata.NewOutgoingContext(ctx, e.metadata)
	}
	return ctx
}

// Send a trace or metrics request to the server. "perform" function is expected to make
// the actual gRPC unary call that sends the request. This function implements the
// common OTLP logic around request handling such as retries and throttling.
func processError(err error) error {
	if err == nil {
		// Request is successful, we are done.
		return nil
	}

	// We have an error, check gRPC status code.

	st := status.Convert(err)
	if st.Code() == codes.OK {
		// Not really an error, still success.
		return nil
	}

	// Now, this is this a real error.

	retryInfo := getRetryInfo(st)

	if !shouldRetry(st.Code(), retryInfo) {
		// It is not a retryable error, we should not retry.
		return consumererror.NewPermanent(err)
	}

	// Check if server returned throttling information.
	throttleDuration := getThrottleDuration(retryInfo)
	if throttleDuration != 0 {
		// We are throttled. Wait before retrying as requested by the server.
		return exporterhelper.NewThrottleRetry(err, throttleDuration)
	}

	// Need to retry.

	return err
}

func shouldRetry(code codes.Code, retryInfo *errdetails.RetryInfo) bool {
	switch code {
	case codes.Canceled,
		codes.DeadlineExceeded,
		codes.Aborted,
		codes.OutOfRange,
		codes.Unavailable,
		codes.DataLoss:
		// These are retryable errors.
		return true
	case codes.ResourceExhausted:
		// Retry only if RetryInfo was supplied by the server.
		// This indicates that the server can still recover from resource exhaustion.
		return retryInfo != nil
	}
	// Don't retry on any other code.
	return false
}

func getRetryInfo(status *status.Status) *errdetails.RetryInfo {
	for _, detail := range status.Details() {
		if t, ok := detail.(*errdetails.RetryInfo); ok {
			return t
		}
	}
	return nil
}

func getThrottleDuration(t *errdetails.RetryInfo) time.Duration {
	if t == nil || t.RetryDelay == nil {
		return 0
	}
	if t.RetryDelay.Seconds > 0 || t.RetryDelay.Nanos > 0 {
		return time.Duration(t.RetryDelay.Seconds)*time.Second + time.Duration(t.RetryDelay.Nanos)*time.Nanosecond
	}
	return 0
}

func (e *exporter) applyMasking(ld plog.Logs) {

	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		resLogs := ld.ResourceLogs().At(i)
		for k := 0; k < resLogs.ScopeLogs().Len(); k++ {
			scopedLog := resLogs.ScopeLogs().At(k)
			for z := 0; z < scopedLog.LogRecords().Len(); z++ {
				log := scopedLog.LogRecords().At(z)
				for _, setting := range e.config.Masking {
					regexp := regexp.MustCompile(setting.Regexp)
					log.Body().SetStringVal(regexp.ReplaceAllString(log.Body().AsString(), setting.Placeholder))
				}
			}
		}
	}

}

func (e *exporter) skipExpired(ld plog.Logs) {
	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		resLogs := ld.ResourceLogs().At(i)

		for k := 0; k < resLogs.ScopeLogs().Len(); k++ {
			resLogs.ScopeLogs().At(k).LogRecords().RemoveIf(func(el plog.LogRecord) bool {
				fmt.Println(el.Timestamp().AsTime().String(), time.Now().Add(-e.config.ExpirationSkip).String())
				if el.Timestamp().AsTime().Before(time.Now().Add(-e.config.ExpirationSkip)) {
					return true
				}
				return false
			})

		}
	}
}
