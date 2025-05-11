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
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"runtime"
	"sync"
	"time"

	_ "github.com/opsramp/go-proxy-dialer/connect" // implemetation for http connect proxy
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	"golang.org/x/net/http/httpproxy"
	"golang.org/x/net/proxy"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

var (
	tokenRenewInProgress bool
	credentials          Credentials
)

type opsrampOTLPExporter struct {
	// Input configuration.
	config *Config

	// gRPC clients and connection.
	traceExporter  ptraceotlp.GRPCClient
	metricExporter pmetricotlp.GRPCClient
	logExporter    plogotlp.GRPCClient
	clientConn     *grpc.ClientConn
	metadata       metadata.MD
	callOptions    []grpc.CallOption

	settings component.TelemetrySettings
	mut      sync.Mutex

	// Default user-agent header.
	userAgent   string
	accessToken string
}

// Crete new exporter and start it. The exporter will begin connecting but
// this function may return before the connection is established.
func newExporter(cfg component.Config, set exporter.Settings) (*opsrampOTLPExporter, error) {
	oCfg := cfg.(*Config)

	accessToken, err := getAuthToken(oCfg.Security)
	if err != nil {
		tokenRenewInProgress = false
		return nil, fmt.Errorf("access token isn't available: %w", err)
	}
	tokenRenewInProgress = false

	if oCfg.Endpoint == "" {
		return nil, errors.New("OTLP exporter config requires an Endpoint")
	}

	userAgent := fmt.Sprintf("%s/%s (%s/%s)",
		set.BuildInfo.Description, set.BuildInfo.Version, runtime.GOOS, runtime.GOARCH)

	return &opsrampOTLPExporter{config: oCfg, settings: set.TelemetrySettings, userAgent: userAgent, accessToken: accessToken}, nil
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
	if tokenRenewInProgress {
		for tokenRenewInProgress {
			time.Sleep(time.Second * 5)
		}
		tokenRenewInProgress = true
		return credentials.AccessToken, nil
	}
	tokenRenewInProgress = true

	client := &http.Client{
		Transport: &http.Transport{
			Proxy: func(req *http.Request) (*url.URL, error) {
				return httpproxy.FromEnvironment().ProxyFunc()(req.URL)
			},
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
	data := url.Values{}
	data.Set("client_id", cfg.ClientID)
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
	jsonResp, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if err := json.Unmarshal(jsonResp, &credentials); err != nil {
		return "", err
	}
	return credentials.AccessToken, nil
}

// start actually creates the gRPC connection. The client construction is deferred till this point as this
// is the only place we get hold of Extensions which are required to construct auth round tripper.
func (e *opsrampOTLPExporter) start(ctx context.Context, host component.Host) (err error) {
	e.clientConn, err = e.config.ClientConfig.ToClientConn(
		ctx,
		host,
		e.settings,
		configgrpc.WithGrpcDialOption(grpc.WithUserAgent(e.userAgent)),
		configgrpc.WithGrpcDialOption(
			grpc.WithContextDialer(func(_ context.Context, addr string) (net.Conn, error) {
				if httpproxy.FromEnvironment().HTTPProxy == "" {
					return (&net.Dialer{}).Dial("tcp", addr)
				}

				uri, er := url.Parse(httpproxy.FromEnvironment().HTTPProxy)
				if er != nil {
					return nil, er
				}

				dialer, er := proxy.FromURL(uri, proxy.Direct)
				if er != nil {
					return nil, er
				}
				return dialer.Dial("tcp", addr)
			})),
	)
	if err != nil {
		return err
	}

	e.traceExporter = ptraceotlp.NewGRPCClient(e.clientConn)
	e.metricExporter = pmetricotlp.NewGRPCClient(e.clientConn)
	e.logExporter = plogotlp.NewGRPCClient(e.clientConn)
	headers := map[string]string{}
	for k, v := range e.config.ClientConfig.Headers {
		headers[k] = string(v)
	}
	e.metadata = metadata.New(headers)
	e.metadata.Set("Authorization", fmt.Sprintf("Bearer %s", e.accessToken))
	e.callOptions = []grpc.CallOption{
		grpc.WaitForReady(e.config.ClientConfig.WaitForReady),
	}
	return
}

func (e *opsrampOTLPExporter) shutdown(context.Context) error {
	if e.clientConn == nil {
		return nil
	}
	return e.clientConn.Close()
}

func (e *opsrampOTLPExporter) pushTraces(ctx context.Context, td ptrace.Traces) error {
	req := ptraceotlp.NewExportRequestFromTraces(td)
	_, err := e.traceExporter.Export(e.enhanceContext(ctx), req, e.callOptions...)
	// trying to get new access token in case of expiration
	if err != nil {
		st := status.Convert(err)
		if st.Code() == codes.Unauthenticated {
			if err = e.updateExpiredToken(); err != nil {
				return fmt.Errorf("couldn't retrieve new token instead of expired: %w", err)
			}
			_, err = e.traceExporter.Export(e.enhanceContext(ctx), req, e.callOptions...)
			if err != nil {
				return err
			}
		}
		return processError(err)
	}
	return nil
}

func (e *opsrampOTLPExporter) pushMetrics(ctx context.Context, md pmetric.Metrics) error {
	req := pmetricotlp.NewExportRequestFromMetrics(md)
	_, err := e.metricExporter.Export(e.enhanceContext(ctx), req, e.callOptions...)
	// trying to get new access token in case of expiration
	if err != nil {
		st := status.Convert(err)
		if st.Code() == codes.Unauthenticated {
			if err = e.updateExpiredToken(); err != nil {
				return fmt.Errorf("couldn't retrieve new token instead of expired: %w", err)
			}

			_, err = e.metricExporter.Export(e.enhanceContext(ctx), req, e.callOptions...)
			if err != nil {
				return err
			}
		}
		return processError(err)
	}

	return processError(err)
}

func (e *opsrampOTLPExporter) pushLogs(_ context.Context, ld plog.Logs) error {
	if ld.LogRecordCount() <= 0 {
		return nil
	}

	if e.config.Masking != nil {
		e.applyMasking(ld)
	}
	if e.config.ExpirationSkip != 0 {
		e.skipExpired(ld)
	}
	if ld.ResourceLogs().Len() <= 0 {
		return nil
	}

	req := plogotlp.NewExportRequestFromLogs(ld)

	_, err := e.logExporter.Export(e.enhanceContext(context.Background()), req, e.callOptions...)
	// trying to get new access token in case of expiration
	if err != nil {
		st := status.Convert(err)
		if st.Code() == codes.Unauthenticated {
			if err = e.updateExpiredToken(); err != nil {
				return fmt.Errorf("couldn't retrieve new token instead of expired: %w", err)
			}

			_, err = e.logExporter.Export(e.enhanceContext(context.Background()), req, e.callOptions...)
			if err != nil {
				return err
			}
		}
		return processError(err)
	}
	return nil
}

func (e *opsrampOTLPExporter) updateExpiredToken() error {
	accessToken, err := getAuthToken(e.config.Security)
	if err != nil {
		tokenRenewInProgress = false
		return err
	}
	e.mut.Lock()
	defer e.mut.Unlock()
	e.accessToken = accessToken
	if tokenRenewInProgress {
		e.metadata.Set("Authorization", fmt.Sprintf("Bearer %s", e.accessToken))
	}
	tokenRenewInProgress = false
	return nil
}

func (e *opsrampOTLPExporter) enhanceContext(ctx context.Context) context.Context {
	e.mut.Lock()
	defer e.mut.Unlock()
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
		codes.Unknown,
		codes.PermissionDenied,
		codes.Internal,
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

func (e *opsrampOTLPExporter) applyMasking(ld plog.Logs) {
	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		resLogs := ld.ResourceLogs().At(i)
		for k := 0; k < resLogs.ScopeLogs().Len(); k++ {
			scopedLog := resLogs.ScopeLogs().At(k)
			for z := 0; z < scopedLog.LogRecords().Len(); z++ {
				log := scopedLog.LogRecords().At(z)
				for _, setting := range e.config.Masking {
					rExp := regexp.MustCompile(setting.Regexp)
					log.Body().SetStr(rExp.ReplaceAllString(log.Body().AsString(), setting.Placeholder))
				}
			}
		}
	}
}

func (e *opsrampOTLPExporter) skipExpired(ld plog.Logs) {
	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		resLogs := ld.ResourceLogs().At(i)

		for k := 0; k < resLogs.ScopeLogs().Len(); k++ {
			resLogs.ScopeLogs().At(k).LogRecords().RemoveIf(func(el plog.LogRecord) bool {
				fmt.Println(el.Timestamp().AsTime().String(), time.Now().Add(-e.config.ExpirationSkip).String())
				return el.Timestamp().AsTime().Before(time.Now().Add(-e.config.ExpirationSkip))
			})
		}
	}
}
