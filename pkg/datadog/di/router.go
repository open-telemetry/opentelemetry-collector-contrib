// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package di // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/di"

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

// Options are resolved by the exporter, never from application attributes.
type Options struct {
	Enabled  bool
	Key      string
	Site     string
	Endpoint string
	HTTP     confighttp.ClientConfig
}

// Router is a pusher placed inside an exporter's queue and retry boundary.
type Router struct {
	options  Options
	settings component.TelemetrySettings
	ordinary consumer.ConsumeLogsFunc
	client   *http.Client
	rejected metric.Int64Counter
}

func New(settings component.TelemetrySettings, options Options, ordinary consumer.ConsumeLogsFunc) (*Router, error) {
	if options.Enabled {
		if options.Endpoint == "" {
			options.Endpoint = "https://debugger-intake." + options.Site + "/api/v2/debugger"
		}
		u, err := url.Parse(options.Endpoint)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return nil, errors.New("DI requires a configured HTTPS intake URL")
		}
		if options.Key == "" || options.HTTP.TLS.Insecure || options.HTTP.TLS.InsecureSkipVerify {
			return nil, errors.New("DI requires API-key authentication and verified TLS")
		}
	}
	counter, err := settings.MeterProvider.Meter("datadog.di").Int64Counter("datadog.di.rejected_records")
	if err != nil {
		return nil, err
	}
	return &Router{options: options, settings: settings, ordinary: ordinary, rejected: counter}, nil
}

func (r *Router) Start(ctx context.Context, host component.Host) error {
	if !r.options.Enabled {
		return nil
	}
	client, err := r.options.HTTP.ToClient(ctx, host.GetExtensions(), r.settings)
	if err != nil {
		return err
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	r.client = client
	return nil
}

func (r *Router) Shutdown(context.Context) error {
	if r.client != nil {
		r.client.CloseIdleConnections()
	}
	return nil
}

func isDI(record plog.LogRecord) bool { return strings.HasPrefix(record.EventName(), "dd.debugger.") }

func (r *Router) ConsumeLogs(ctx context.Context, input plog.Logs) error {
	if !containsDI(input) {
		return r.ordinary(ctx, input)
	}
	di, ordinary := split(input)
	failed, rejected := plog.NewLogs(), plog.NewLogs()
	var retryErr, rejectErr error
	for i := 0; i < di.ResourceLogs().Len(); i++ {
		rl := di.ResourceLogs().At(i)
		for j := 0; j < rl.ScopeLogs().Len(); j++ {
			sl := rl.ScopeLogs().At(j)
			for k := 0; k < sl.LogRecords().Len(); k++ {
				record := sl.LogRecords().At(k)
				err := r.send(ctx, rl, record)
				if err == nil {
					continue
				}
				if consumererror.IsPermanent(err) {
					copyRecord(rejected, rl, sl, record)
					rejectErr = err
				} else {
					copyRecord(failed, rl, sl, record)
					retryErr = err
				}
			}
		}
	}
	if ordinary.LogRecordCount() > 0 {
		if err := r.ordinary(ctx, ordinary); err != nil {
			remaining := ordinary
			if partial, ok := errors.AsType[consumererror.Logs](err); ok {
				remaining = partial.Data()
			}
			if consumererror.IsPermanent(err) {
				remaining.ResourceLogs().MoveAndAppendTo(rejected.ResourceLogs())
				rejectErr = err
			} else {
				remaining.ResourceLogs().MoveAndAppendTo(failed.ResourceLogs())
				retryErr = err
			}
		}
	}
	if rejected.LogRecordCount() > 0 {
		r.rejected.Add(ctx, int64(rejected.LogRecordCount()))
		r.settings.Logger.Warn("Rejected invalid or permanently failed records", zap.Int("records", rejected.LogRecordCount()))
	}
	if retryErr != nil {
		return consumererror.NewLogs(retryErr, failed)
	}
	if rejectErr != nil {
		return consumererror.NewPermanent(consumererror.NewLogs(rejectErr, rejected))
	}
	return nil
}

func (r *Router) send(ctx context.Context, resource plog.ResourceLogs, record plog.LogRecord) error {
	if !r.options.Enabled || r.client == nil {
		return consumererror.NewPermanent(errors.New("DI delivery is disabled or not started"))
	}
	body, contentType, err := encodeDIEvent(record)
	if err != nil {
		return consumererror.NewPermanent(err)
	}
	tags, err := nativeTags(resource, record)
	if err != nil {
		return consumererror.NewPermanent(err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.options.Endpoint, bytes.NewReader(body))
	if err != nil {
		return consumererror.NewPermanent(errors.New("invalid DI intake request"))
	}
	req.Header.Set("DD-API-KEY", r.options.Key)
	req.Header.Set("DD-REQUEST-ID", uuid.NewString())
	req.Header.Set("Content-Type", contentType)
	if record.EventName() == "dd.debugger.symbols" {
		req.Header.Set("DD-EVP-ORIGIN", "agent-symdb")
		req.Header.Set("X-Datadog-Additional-Tags", tags)
	} else {
		req.Header.Set("DD-EVP-ORIGIN", "agent-debugger")
		req.URL.RawQuery = url.Values{"ddtags": {tags}}.Encode()
	}
	response, err := r.client.Do(req)
	if err != nil {
		return errors.New("DI intake transport failed")
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return nil
	}
	err = fmt.Errorf("DI intake returned HTTP %d", response.StatusCode)
	if response.StatusCode < http.StatusInternalServerError && response.StatusCode != http.StatusRequestTimeout && response.StatusCode != http.StatusTooManyRequests {
		return consumererror.NewPermanent(err)
	}
	return err
}

func nativeTags(resource plog.ResourceLogs, record plog.LogRecord) (string, error) {
	if value, ok := resource.Resource().Attributes().Get("service.name"); ok {
		var payload struct {
			Service string `json:"service"`
		}
		raw, _ := record.Attributes().Get("datadog.di.payload")
		if err := json.Unmarshal([]byte(raw.Str()), &payload); err != nil {
			return "", err
		}
		if payload.Service != "" && payload.Service != value.AsString() {
			return "", errors.New("conflicting DI payload and resource service")
		}
	}

	values := []string{}
	tags := map[string][]string{}
	if v, ok := record.Attributes().Get("datadog.di.tags"); ok {
		if v.Type() != pcommon.ValueTypeStr {
			return "", errors.New("DI tags must be a string")
		}
		for tag := range strings.SplitSeq(v.Str(), ",") {
			if tag == "" {
				continue
			}
			if strings.ContainsAny(tag, "\r\n") {
				return "", errors.New("DI tags contain a line break")
			}
			values = append(values, tag)
			if key, value, ok := strings.Cut(tag, ":"); ok {
				tags[key] = append(tags[key], value)
			}
		}
	}
	for _, mapping := range [][2]string{{"host.name", "host"}, {"container.id", "container_id"}, {"deployment.environment.name", "env"}, {"service.version", "version"}} {
		if value, ok := resource.Resource().Attributes().Get(mapping[0]); ok && value.AsString() != "" {
			if strings.ContainsAny(value.AsString(), ",\r\n") {
				return "", errors.New("invalid DI resource tag value")
			}
			existing := tags[mapping[1]]
			for _, old := range existing {
				if old != value.AsString() {
					return "", errors.New("conflicting DI resource identity and SDK tags")
				}
			}
			if len(existing) == 0 {
				values = append(values, mapping[1]+":"+value.AsString())
			}
		}
	}
	return strings.Join(values, ","), nil
}

func containsDI(input plog.Logs) bool {
	for i := 0; i < input.ResourceLogs().Len(); i++ {
		groups := input.ResourceLogs().At(i).ScopeLogs()
		for j := 0; j < groups.Len(); j++ {
			for k := 0; k < groups.At(j).LogRecords().Len(); k++ {
				if isDI(groups.At(j).LogRecords().At(k)) {
					return true
				}
			}
		}
	}
	return false
}

func split(input plog.Logs) (plog.Logs, plog.Logs) {
	di, ordinary := plog.NewLogs(), plog.NewLogs()
	for i := 0; i < input.ResourceLogs().Len(); i++ {
		rl := input.ResourceLogs().At(i)
		var destinations [2]plog.ResourceLogs
		var resourceCreated [2]bool
		for j := 0; j < rl.ScopeLogs().Len(); j++ {
			sl := rl.ScopeLogs().At(j)
			var scopes [2]plog.ScopeLogs
			var scopeCreated [2]bool
			for k := 0; k < sl.LogRecords().Len(); k++ {
				record := sl.LogRecords().At(k)
				index, destination := 0, ordinary
				if isDI(record) {
					index, destination = 1, di
				}
				if !resourceCreated[index] {
					destinations[index] = destination.ResourceLogs().AppendEmpty()
					rl.Resource().CopyTo(destinations[index].Resource())
					destinations[index].SetSchemaUrl(rl.SchemaUrl())
					resourceCreated[index] = true
				}
				if !scopeCreated[index] {
					scopes[index] = destinations[index].ScopeLogs().AppendEmpty()
					sl.Scope().CopyTo(scopes[index].Scope())
					scopes[index].SetSchemaUrl(sl.SchemaUrl())
					scopeCreated[index] = true
				}
				record.CopyTo(scopes[index].LogRecords().AppendEmpty())
			}
		}
	}
	return di, ordinary
}

func copyRecord(dest plog.Logs, resource plog.ResourceLogs, scope plog.ScopeLogs, record plog.LogRecord) {
	rl := dest.ResourceLogs().AppendEmpty()
	resource.Resource().CopyTo(rl.Resource())
	rl.SetSchemaUrl(resource.SchemaUrl())
	sl := rl.ScopeLogs().AppendEmpty()
	scope.Scope().CopyTo(sl.Scope())
	sl.SetSchemaUrl(scope.SchemaUrl())
	record.CopyTo(sl.LogRecords().AppendEmpty())
}
