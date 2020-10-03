// Copyright 2020, OpenTelemetry Authors
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

package elastic_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.elastic.co/apm/model"
	"go.elastic.co/apm/transport/transporttest"
	"go.elastic.co/fastjson"
	"go.opentelemetry.io/collector/consumer/pdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticexporter/internal/translator/elastic"
)

func TestEncodeResourceSpan(t *testing.T) {
	var w fastjson.Writer
	var recorder transporttest.RecorderTransport
	elastic.EncodeResourceMetadata(pdata.NewResource(), &w)

	traceID := model.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	rootTransactionID := model.SpanID{1, 1, 1, 1, 1, 1, 1, 1}
	clientSpanID := model.SpanID{2, 2, 2, 2, 2, 2, 2, 2}
	serverTransactionID := model.SpanID{3, 3, 3, 3, 3, 3, 3, 3}

	startTime := time.Unix(123, 0).UTC()
	endTime := startTime.Add(time.Millisecond * 5)

	rootSpan := pdata.NewSpan()
	rootSpan.InitEmpty()
	rootSpan.SetSpanID(pdata.NewSpanID(rootTransactionID[:]))
	rootSpan.SetName("root_span")
	rootSpan.Attributes().InitFromMap(map[string]pdata.AttributeValue{
		"string.attr": pdata.NewAttributeValueString("string_value"),
		"int.attr":    pdata.NewAttributeValueInt(123),
		"double.attr": pdata.NewAttributeValueDouble(123.456),
		"bool.attr":   pdata.NewAttributeValueBool(true),
	})

	clientSpan := pdata.NewSpan()
	clientSpan.InitEmpty()
	clientSpan.SetSpanID(pdata.NewSpanID(clientSpanID[:]))
	clientSpan.SetParentSpanID(pdata.NewSpanID(rootTransactionID[:]))
	clientSpan.SetKind(pdata.SpanKindCLIENT)
	clientSpan.SetName("client_span")
	clientSpan.Status().InitEmpty()
	clientSpan.Attributes().InitFromMap(map[string]pdata.AttributeValue{
		"string.attr": pdata.NewAttributeValueString("string_value"),
		"int.attr":    pdata.NewAttributeValueInt(123),
		"double.attr": pdata.NewAttributeValueDouble(123.456),
		"bool.attr":   pdata.NewAttributeValueBool(true),
	})

	serverSpan := pdata.NewSpan()
	serverSpan.InitEmpty()
	serverSpan.SetSpanID(pdata.NewSpanID(serverTransactionID[:]))
	serverSpan.SetParentSpanID(pdata.NewSpanID(clientSpanID[:]))
	serverSpan.SetKind(pdata.SpanKindSERVER)
	serverSpan.SetName("server_span")
	serverSpan.Status().InitEmpty()
	serverSpan.Status().SetCode(-1)

	for _, span := range []pdata.Span{rootSpan, clientSpan, serverSpan} {
		span.SetTraceID(pdata.NewTraceID(traceID[:]))
		span.SetStartTime(pdata.TimestampUnixNano(startTime.UnixNano()))
		span.SetEndTime(pdata.TimestampUnixNano(endTime.UnixNano()))
	}

	for _, span := range []pdata.Span{rootSpan, clientSpan, serverSpan} {
		err := elastic.EncodeSpan(span, pdata.NewInstrumentationLibrary(), &w)
		require.NoError(t, err)
	}
	sendStream(t, &w, &recorder)

	payloads := recorder.Payloads()
	assert.Equal(t, []model.Transaction{{
		TraceID:   traceID,
		ID:        rootTransactionID,
		Timestamp: model.Time(startTime),
		Duration:  5.0,
		Name:      "root_span",
		Type:      "unknown",
		Result:    "STATUS_CODE_OK",
		Context: &model.Context{
			Tags: model.IfaceMap{{
				Key:   "bool_attr",
				Value: true,
			}, {
				Key:   "double_attr",
				Value: 123.456,
			}, {
				Key:   "int_attr",
				Value: float64(123),
			}, {
				Key:   "string_attr",
				Value: "string_value",
			}},
		},
	}, {
		TraceID:   traceID,
		ID:        serverTransactionID,
		ParentID:  clientSpanID,
		Timestamp: model.Time(startTime),
		Duration:  5.0,
		Name:      "server_span",
		Type:      "unknown",
		Result:    "-1",
	}}, payloads.Transactions)

	assert.Equal(t, []model.Span{{
		TraceID:   traceID,
		ID:        clientSpanID,
		ParentID:  rootTransactionID,
		Timestamp: model.Time(startTime),
		Duration:  5.0,
		Name:      "client_span",
		Type:      "app",
		Context: &model.SpanContext{
			Tags: model.IfaceMap{{
				Key:   "bool_attr",
				Value: true,
			}, {
				Key:   "double_attr",
				Value: 123.456,
			}, {
				Key:   "int_attr",
				Value: float64(123),
			}, {
				Key:   "string_attr",
				Value: "string_value",
			}},
		},
	}}, payloads.Spans)

	assert.Empty(t, payloads.Errors)
}

func TestTransactionHTTPRequestURL(t *testing.T) {
	test := func(t *testing.T, expectedFull string, attrs map[string]pdata.AttributeValue) {
		transaction := transactionWithAttributes(t, attrs)
		assert.Equal(t, expectedFull, transaction.Context.Request.URL.Full)
	}
	t.Run("scheme_host_target", func(t *testing.T) {
		test(t, "https://testing.invalid:80/foo?bar", map[string]pdata.AttributeValue{
			"http.scheme": pdata.NewAttributeValueString("https"),
			"http.host":   pdata.NewAttributeValueString("testing.invalid:80"),
			"http.target": pdata.NewAttributeValueString("/foo?bar"),
		})
	})
	t.Run("scheme_servername_nethostport_target", func(t *testing.T) {
		test(t, "https://testing.invalid:80/foo?bar", map[string]pdata.AttributeValue{
			"http.scheme":      pdata.NewAttributeValueString("https"),
			"http.server_name": pdata.NewAttributeValueString("testing.invalid"),
			"net.host.port":    pdata.NewAttributeValueInt(80),
			"http.target":      pdata.NewAttributeValueString("/foo?bar"),
		})
	})
	t.Run("scheme_nethostname_nethostport_target", func(t *testing.T) {
		test(t, "https://testing.invalid:80/foo?bar", map[string]pdata.AttributeValue{
			"http.scheme":   pdata.NewAttributeValueString("https"),
			"net.host.name": pdata.NewAttributeValueString("testing.invalid"),
			"net.host.port": pdata.NewAttributeValueInt(80),
			"http.target":   pdata.NewAttributeValueString("/foo?bar"),
		})
	})
	t.Run("http.url", func(t *testing.T) {
		const httpURL = "https://testing.invalid:80/foo?bar"
		test(t, httpURL, map[string]pdata.AttributeValue{
			"http.url": pdata.NewAttributeValueString(httpURL),
		})
	})
	t.Run("host_no_port", func(t *testing.T) {
		test(t, "https://testing.invalid/foo?bar", map[string]pdata.AttributeValue{
			"http.scheme": pdata.NewAttributeValueString("https"),
			"http.host":   pdata.NewAttributeValueString("testing.invalid"),
			"http.target": pdata.NewAttributeValueString("/foo?bar"),
		})
	})
	t.Run("ipv6_host_no_port", func(t *testing.T) {
		test(t, "https://[::1]/foo?bar", map[string]pdata.AttributeValue{
			"http.scheme": pdata.NewAttributeValueString("https"),
			"http.host":   pdata.NewAttributeValueString("[::1]"),
			"http.target": pdata.NewAttributeValueString("/foo?bar"),
		})
	})

	// Scheme is set to "http" if it can't be deduced from attributes.
	t.Run("default_scheme", func(t *testing.T) {
		test(t, "http://testing.invalid:80/foo?bar", map[string]pdata.AttributeValue{
			"http.host":   pdata.NewAttributeValueString("testing.invalid:80"),
			"http.target": pdata.NewAttributeValueString("/foo?bar"),
		})
	})
}

func TestTransactionHTTPRequestURLInvalid(t *testing.T) {
	transaction := transactionWithAttributes(t, map[string]pdata.AttributeValue{
		"http.url": pdata.NewAttributeValueString("0.0.0.0:8081"),
	})
	require.NotNil(t, transaction.Context)
	assert.Nil(t, transaction.Context.Request)
	assert.Equal(t, model.IfaceMap{
		{Key: "http_url", Value: "0.0.0.0:8081"},
	}, transaction.Context.Tags)
}

func TestTransactionHTTPRequestSocketRemoteAddr(t *testing.T) {
	test := func(t *testing.T, expected string, attrs map[string]pdata.AttributeValue) {
		transaction := transactionWithAttributes(t, attrs)
		assert.Equal(t, expected, transaction.Context.Request.Socket.RemoteAddress)
	}
	t.Run("net.peer.ip_port", func(t *testing.T) {
		test(t, "192.168.0.1:1234", map[string]pdata.AttributeValue{
			"http.url":      pdata.NewAttributeValueString("http://testing.invalid"),
			"net.peer.ip":   pdata.NewAttributeValueString("192.168.0.1"),
			"net.peer.port": pdata.NewAttributeValueInt(1234),
		})
	})
	t.Run("net.peer.ip", func(t *testing.T) {
		test(t, "192.168.0.1", map[string]pdata.AttributeValue{
			"http.url":    pdata.NewAttributeValueString("http://testing.invalid"),
			"net.peer.ip": pdata.NewAttributeValueString("192.168.0.1"),
		})
	})
	t.Run("http.remote_addr", func(t *testing.T) {
		test(t, "192.168.0.1:1234", map[string]pdata.AttributeValue{
			"http.url":         pdata.NewAttributeValueString("http://testing.invalid"),
			"http.remote_addr": pdata.NewAttributeValueString("192.168.0.1:1234"),
		})
	})
	t.Run("http.remote_addr_no_port", func(t *testing.T) {
		test(t, "192.168.0.1", map[string]pdata.AttributeValue{
			"http.url":         pdata.NewAttributeValueString("http://testing.invalid"),
			"http.remote_addr": pdata.NewAttributeValueString("192.168.0.1"),
		})
	})
}

func TestTransactionHTTPRequestHTTPVersion(t *testing.T) {
	transaction := transactionWithAttributes(t, map[string]pdata.AttributeValue{
		"http.flavor": pdata.NewAttributeValueString("1.1"),
	})
	assert.Equal(t, "1.1", transaction.Context.Request.HTTPVersion)
}

func TestTransactionHTTPRequestHTTPMethod(t *testing.T) {
	transaction := transactionWithAttributes(t, map[string]pdata.AttributeValue{
		"http.method": pdata.NewAttributeValueString("PATCH"),
	})
	assert.Equal(t, "PATCH", transaction.Context.Request.Method)
}

func TestTransactionHTTPRequestUserAgent(t *testing.T) {
	transaction := transactionWithAttributes(t, map[string]pdata.AttributeValue{
		"http.user_agent": pdata.NewAttributeValueString("Foo/bar (baz)"),
	})
	assert.Equal(t, model.Headers{{
		Key:    "User-Agent",
		Values: []string{"Foo/bar (baz)"},
	}}, transaction.Context.Request.Headers)
}

func TestTransactionHTTPRequestClientIP(t *testing.T) {
	transaction := transactionWithAttributes(t, map[string]pdata.AttributeValue{
		"http.client_ip": pdata.NewAttributeValueString("256.257.258.259"),
	})
	assert.Equal(t, model.Headers{{
		Key:    "X-Forwarded-For",
		Values: []string{"256.257.258.259"},
	}}, transaction.Context.Request.Headers)
}

func TestTransactionHTTPResponseStatusCode(t *testing.T) {
	transaction := transactionWithAttributes(t, map[string]pdata.AttributeValue{
		"http.status_code": pdata.NewAttributeValueInt(200),
	})
	assert.Equal(t, 200, transaction.Context.Response.StatusCode)
}

func TestSpanHTTPURL(t *testing.T) {
	test := func(t *testing.T, expectedURL string, attrs map[string]pdata.AttributeValue) {
		span := spanWithAttributes(t, attrs)
		assert.Equal(t, expectedURL, span.Context.HTTP.URL.String())
	}
	t.Run("http.url", func(t *testing.T) {
		const httpURL = "https://testing.invalid:80/foo?bar"
		test(t, httpURL, map[string]pdata.AttributeValue{
			"http.url": pdata.NewAttributeValueString(httpURL),
		})
	})
	t.Run("scheme_host_target", func(t *testing.T) {
		test(t, "https://testing.invalid:80/foo?bar", map[string]pdata.AttributeValue{
			"http.scheme": pdata.NewAttributeValueString("https"),
			"http.host":   pdata.NewAttributeValueString("testing.invalid:80"),
			"http.target": pdata.NewAttributeValueString("/foo?bar"),
		})
	})
	t.Run("scheme_netpeername_netpeerport_target", func(t *testing.T) {
		test(t, "https://testing.invalid:80/foo?bar", map[string]pdata.AttributeValue{
			"http.scheme":   pdata.NewAttributeValueString("https"),
			"net.peer.name": pdata.NewAttributeValueString("testing.invalid"),
			"net.peer.ip":   pdata.NewAttributeValueString("::1"), // net.peer.name preferred
			"net.peer.port": pdata.NewAttributeValueInt(80),
			"http.target":   pdata.NewAttributeValueString("/foo?bar"),
		})
	})
	t.Run("scheme_netpeerip_netpeerport_target", func(t *testing.T) {
		test(t, "https://[::1]:80/foo?bar", map[string]pdata.AttributeValue{
			"http.scheme":   pdata.NewAttributeValueString("https"),
			"net.peer.ip":   pdata.NewAttributeValueString("::1"),
			"net.peer.port": pdata.NewAttributeValueInt(80),
			"http.target":   pdata.NewAttributeValueString("/foo?bar"),
		})
	})

	// Scheme is set to "http" if it can't be deduced from attributes.
	t.Run("default_scheme", func(t *testing.T) {
		test(t, "http://testing.invalid:80/foo?bar", map[string]pdata.AttributeValue{
			"http.host":   pdata.NewAttributeValueString("testing.invalid:80"),
			"http.target": pdata.NewAttributeValueString("/foo?bar"),
		})
	})
}

func TestSpanHTTPDestination(t *testing.T) {
	test := func(t *testing.T, expectedAddr string, expectedPort int, expectedName string, expectedResource string, attrs map[string]pdata.AttributeValue) {
		span := spanWithAttributes(t, attrs)
		assert.Equal(t, &model.DestinationSpanContext{
			Address: expectedAddr,
			Port:    expectedPort,
			Service: &model.DestinationServiceSpanContext{
				Type:     "external",
				Name:     expectedName,
				Resource: expectedResource,
			},
		}, span.Context.Destination)
	}
	t.Run("url_default_port_specified", func(t *testing.T) {
		test(t, "testing.invalid", 443, "https://testing.invalid", "testing.invalid:443", map[string]pdata.AttributeValue{
			"http.url": pdata.NewAttributeValueString("https://testing.invalid:443/foo?bar"),
		})
	})
	t.Run("url_port_scheme", func(t *testing.T) {
		test(t, "testing.invalid", 443, "https://testing.invalid", "testing.invalid:443", map[string]pdata.AttributeValue{
			"http.url": pdata.NewAttributeValueString("https://testing.invalid/foo?bar"),
		})
	})
	t.Run("url_non_default_port", func(t *testing.T) {
		test(t, "testing.invalid", 444, "https://testing.invalid:444", "testing.invalid:444", map[string]pdata.AttributeValue{
			"http.url": pdata.NewAttributeValueString("https://testing.invalid:444/foo?bar"),
		})
	})
	t.Run("scheme_host_target", func(t *testing.T) {
		test(t, "testing.invalid", 444, "https://testing.invalid:444", "testing.invalid:444", map[string]pdata.AttributeValue{
			"http.scheme": pdata.NewAttributeValueString("https"),
			"http.host":   pdata.NewAttributeValueString("testing.invalid:444"),
			"http.target": pdata.NewAttributeValueString("/foo?bar"),
		})
	})
	t.Run("scheme_netpeername_nethostport_target", func(t *testing.T) {
		test(t, "::1", 444, "https://[::1]:444", "[::1]:444", map[string]pdata.AttributeValue{
			"http.scheme":   pdata.NewAttributeValueString("https"),
			"net.peer.ip":   pdata.NewAttributeValueString("::1"),
			"net.peer.port": pdata.NewAttributeValueInt(444),
			"http.target":   pdata.NewAttributeValueString("/foo?bar"),
		})
	})
}

func TestSpanHTTPURLInvalid(t *testing.T) {
	span := spanWithAttributes(t, map[string]pdata.AttributeValue{
		"http.url": pdata.NewAttributeValueString("0.0.0.0:8081"),
	})
	require.NotNil(t, span.Context)
	assert.Nil(t, span.Context.HTTP)
	assert.Equal(t, model.IfaceMap{
		{Key: "http_url", Value: "0.0.0.0:8081"},
	}, span.Context.Tags)
}

func TestSpanHTTPStatusCode(t *testing.T) {
	span := spanWithAttributes(t, map[string]pdata.AttributeValue{
		"http.status_code": pdata.NewAttributeValueInt(200),
	})
	assert.Equal(t, 200, span.Context.HTTP.StatusCode)
}

func TestSpanDatabaseContext(t *testing.T) {
	// https://github.com/open-telemetry/opentelemetry-specification/blob/master/specification/trace/semantic_conventions/database.md#mysql
	connectionString := "Server=shopdb.example.com;Database=ShopDb;Uid=billing_user;TableCache=true;UseCompression=True;MinimumPoolSize=10;MaximumPoolSize=50;"
	span := spanWithAttributes(t, map[string]pdata.AttributeValue{
		"db.system":            pdata.NewAttributeValueString("mysql"),
		"db.connection_string": pdata.NewAttributeValueString(connectionString),
		"db.user":              pdata.NewAttributeValueString("billing_user"),
		"db.name":              pdata.NewAttributeValueString("ShopDb"),
		"db.statement":         pdata.NewAttributeValueString("SELECT * FROM orders WHERE order_id = 'o4711'"),
		"net.peer.name":        pdata.NewAttributeValueString("shopdb.example.com"),
		"net.peer.ip":          pdata.NewAttributeValueString("192.0.2.12"),
		"net.peer.port":        pdata.NewAttributeValueInt(3306),
		"net.transport":        pdata.NewAttributeValueString("IP.TCP"),
	})

	assert.Equal(t, "db", span.Type)
	assert.Equal(t, "mysql", span.Subtype)
	assert.Equal(t, "", span.Action)

	assert.Equal(t, &model.DatabaseSpanContext{
		Type:      "mysql",
		Instance:  "ShopDb",
		Statement: "SELECT * FROM orders WHERE order_id = 'o4711'",
		User:      "billing_user",
	}, span.Context.Database)

	assert.Equal(t, model.IfaceMap{
		{Key: "db_connection_string", Value: connectionString},
		{Key: "net_transport", Value: "IP.TCP"},
	}, span.Context.Tags)

	assert.Equal(t, &model.DestinationSpanContext{
		Address: "shopdb.example.com",
		Port:    3306,
		Service: &model.DestinationServiceSpanContext{
			Type:     "db",
			Name:     "mysql",
			Resource: "mysql",
		},
	}, span.Context.Destination)
}

func TestInstrumentationLibrary(t *testing.T) {
	var w fastjson.Writer
	var recorder transporttest.RecorderTransport

	span := pdata.NewSpan()
	span.InitEmpty()
	span.SetName("root_span")

	library := pdata.NewInstrumentationLibrary()
	library.InitEmpty()
	library.SetName("library-name")
	library.SetVersion("1.2.3")

	elastic.EncodeResourceMetadata(pdata.NewResource(), &w)
	err := elastic.EncodeSpan(span, library, &w)
	assert.NoError(t, err)
	sendStream(t, &w, &recorder)

	payloads := recorder.Payloads()
	require.Len(t, payloads.Transactions, 1)
	assert.Equal(t, &model.Context{
		Service: &model.Service{
			Framework: &model.Framework{
				Name:    "library-name",
				Version: "1.2.3",
			},
		},
	}, payloads.Transactions[0].Context)
}

func transactionWithAttributes(t *testing.T, attrs map[string]pdata.AttributeValue) model.Transaction {
	var w fastjson.Writer
	var recorder transporttest.RecorderTransport

	span := pdata.NewSpan()
	span.InitEmpty()
	span.Attributes().InitFromMap(attrs)

	elastic.EncodeResourceMetadata(pdata.NewResource(), &w)
	err := elastic.EncodeSpan(span, pdata.NewInstrumentationLibrary(), &w)
	assert.NoError(t, err)
	sendStream(t, &w, &recorder)

	payloads := recorder.Payloads()
	require.Len(t, payloads.Transactions, 1)
	return payloads.Transactions[0]
}

func spanWithAttributes(t *testing.T, attrs map[string]pdata.AttributeValue) model.Span {
	var w fastjson.Writer
	var recorder transporttest.RecorderTransport

	span := pdata.NewSpan()
	span.InitEmpty()
	span.SetParentSpanID(pdata.NewSpanID([]byte{1}))
	span.Attributes().InitFromMap(attrs)

	elastic.EncodeResourceMetadata(pdata.NewResource(), &w)
	err := elastic.EncodeSpan(span, pdata.NewInstrumentationLibrary(), &w)
	assert.NoError(t, err)
	sendStream(t, &w, &recorder)

	payloads := recorder.Payloads()
	require.Len(t, payloads.Spans, 1)
	return payloads.Spans[0]
}
