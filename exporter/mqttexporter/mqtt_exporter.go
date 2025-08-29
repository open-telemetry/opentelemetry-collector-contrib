// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mqttexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/mqttexporter"

import (
	"context"
	"crypto/tls"
	"net/url"
	"regexp"
	"strings"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/mqttexporter/internal/publisher"
)

type mqttExporter struct {
	config *Config
	tlsFactory
	settings component.TelemetrySettings
	topic    string
	clientID string
	*marshaler
	publisherFactory
	publisher publisher.Publisher
}

type (
	publisherFactory = func(publisher.DialConfig) (publisher.Publisher, error)
	tlsFactory       = func(context.Context) (*tls.Config, error)
)

func newMqttExporter(cfg *Config, set component.TelemetrySettings, publisherFactory publisherFactory, tlsFactory tlsFactory, topic, clientID string) *mqttExporter {
	exporter := &mqttExporter{
		config:           cfg,
		settings:         set,
		topic:            topic,
		clientID:         clientID,
		publisherFactory: publisherFactory,
		tlsFactory:       tlsFactory,
	}
	return exporter
}

func (e *mqttExporter) start(ctx context.Context, host component.Host) error {
	m, err := newMarshaler(e.config.EncodingExtensionID, host)
	if err != nil {
		return err
	}
	e.marshaler = m

	brokerURL, err := url.Parse(e.config.Connection.Endpoint)
	if err != nil {
		return err
	}

	dialConfig := publisher.DialConfig{
		ClientOptions: mqtt.ClientOptions{
			Servers:        []*url.URL{brokerURL},
			ClientID:       e.clientID,
			Username:       e.config.Connection.Auth.Plain.Username,
			Password:       e.config.Connection.Auth.Plain.Password,
			ConnectTimeout: e.config.Connection.ConnectionTimeout,
			KeepAlive:      int64(e.config.Connection.KeepAlive.Seconds()),
		},
		QoS:                        e.config.QoS,
		Retain:                     e.config.Retain,
		PublishConfirmationTimeout: e.config.Connection.PublishConfirmationTimeout,
	}

	tlsConfig, err := e.tlsFactory(ctx)
	if err != nil {
		return err
	}
	if tlsConfig != nil {
		dialConfig.TLS = tlsConfig
	}

	e.settings.Logger.Info("Establishing initial connection to MQTT broker")
	p, err := e.publisherFactory(dialConfig)
	e.publisher = p

	if err != nil {
		return err
	}

	return nil
}

func (e *mqttExporter) publishTraces(ctx context.Context, traces ptrace.Traces) error {
	body, err := e.tracesMarshaler.MarshalTraces(traces)
	if err != nil {
		return err
	}

	topic := renderTopicFromTraces(e.topic, traces)
	if topic == "" {
		topic = e.topic
	}
	message := publisher.Message{
		Topic: topic,
		Body:  body,
	}
	return e.publisher.Publish(ctx, message)
}

func (e *mqttExporter) publishMetrics(ctx context.Context, metrics pmetric.Metrics) error {
	body, err := e.metricsMarshaler.MarshalMetrics(metrics)
	if err != nil {
		return err
	}

	topic := renderTopicFromMetrics(e.topic, metrics)
	if topic == "" {
		topic = e.topic
	}
	message := publisher.Message{
		Topic: topic,
		Body:  body,
	}
	return e.publisher.Publish(ctx, message)
}

func (e *mqttExporter) publishLogs(ctx context.Context, logs plog.Logs) error {
	body, err := e.logsMarshaler.MarshalLogs(logs)
	if err != nil {
		return err
	}

	topic := renderTopicFromLogs(e.topic, logs)
	if topic == "" {
		topic = e.topic
	}
	message := publisher.Message{
		Topic: topic,
		Body:  body,
	}
	return e.publisher.Publish(ctx, message)
}

func (e *mqttExporter) shutdown(_ context.Context) error {
	if e.publisher != nil {
		return e.publisher.Close()
	}
	return nil
}

// Template rendering helpers
// Supports placeholders like ${resource.attributes.host.name}
var resAttrPattern = regexp.MustCompile(`\$\{resource\.attributes\.([a-zA-Z0-9_.-]+)\}`)

func renderWithResource(template string, getAttr func(string) (string, bool)) string {
	if template == "" {
		return template
	}
	return resAttrPattern.ReplaceAllStringFunc(template, func(m string) string {
		match := resAttrPattern.FindStringSubmatch(m)
		if len(match) != 2 {
			return ""
		}
		key := match[1]
		// Attribute keys in OTel are dot-separated (e.g., host.name)
		if v, ok := getAttr(key); ok {
			return sanitizeTopicFragment(v)
		}
		return ""
	})
}

func renderTopicFromMetrics(template string, md pmetric.Metrics) string {
	if md.ResourceMetrics().Len() == 0 {
		return template
	}
	rm := md.ResourceMetrics().At(0)
	attrs := rm.Resource().Attributes()
	return renderWithResource(template, func(k string) (string, bool) {
		if v, ok := attrs.Get(k); ok {
			return v.AsString(), true
		}
		return "", false
	})
}

func renderTopicFromTraces(template string, td ptrace.Traces) string {
	if td.ResourceSpans().Len() == 0 {
		return template
	}
	rs := td.ResourceSpans().At(0)
	attrs := rs.Resource().Attributes()
	return renderWithResource(template, func(k string) (string, bool) {
		if v, ok := attrs.Get(k); ok {
			return v.AsString(), true
		}
		return "", false
	})
}

func renderTopicFromLogs(template string, ld plog.Logs) string {
	if ld.ResourceLogs().Len() == 0 {
		return template
	}
	rl := ld.ResourceLogs().At(0)
	attrs := rl.Resource().Attributes()
	return renderWithResource(template, func(k string) (string, bool) {
		if v, ok := attrs.Get(k); ok {
			return v.AsString(), true
		}
		return "", false
	})
}

// sanitizeTopicFragment ensures the substituted value is safe for MQTT topic segments
func sanitizeTopicFragment(s string) string {
	// Disallow wildcard and control characters in topics; replace with '-'
	s = strings.ReplaceAll(s, "+", "-")
	s = strings.ReplaceAll(s, "#", "-")
	s = strings.ReplaceAll(s, "\u0000", "-")
	return s
}
