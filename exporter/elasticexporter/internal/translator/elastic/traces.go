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

// Package elastic contains an opentelemetry-collector exporter
// for Elastic APM.
package elastic

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"time"

	"go.elastic.co/apm/model"
	"go.elastic.co/fastjson"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/conventions"
)

// EncodeSpan encodes an OpenTelemetry span, and instrumentation library information,
// as a transaction or span line, writing to w.
func EncodeSpan(otlpSpan pdata.Span, otlpLibrary pdata.InstrumentationLibrary, w *fastjson.Writer) error {
	var spanID, parentID model.SpanID
	var traceID model.TraceID
	copy(spanID[:], otlpSpan.SpanID().Bytes())
	copy(traceID[:], otlpSpan.TraceID().Bytes())
	copy(parentID[:], otlpSpan.ParentSpanID().Bytes())
	root := parentID == model.SpanID{}

	startTime := time.Unix(0, int64(otlpSpan.StartTime())).UTC()
	endTime := time.Unix(0, int64(otlpSpan.EndTime())).UTC()
	durationMillis := endTime.Sub(startTime).Seconds() * 1000

	name := otlpSpan.Name()
	var transactionContext transactionContext
	if root || otlpSpan.Kind() == pdata.SpanKindSERVER {
		transaction := model.Transaction{
			ID:        spanID,
			TraceID:   traceID,
			ParentID:  parentID,
			Name:      name,
			Timestamp: model.Time(startTime),
			Duration:  durationMillis,
		}
		if err := setTransactionProperties(
			otlpSpan, otlpLibrary,
			&transaction, &transactionContext,
		); err != nil {
			return err
		}
		transaction.Context = transactionContext.modelContext()
		w.RawString(`{"transaction":`)
		if err := transaction.MarshalFastJSON(w); err != nil {
			return err
		}
		w.RawString("}\n")
	} else {
		span := model.Span{
			ID:        spanID,
			TraceID:   traceID,
			ParentID:  parentID,
			Timestamp: model.Time(startTime),
			Duration:  durationMillis,
			Name:      name,
		}
		if err := setSpanProperties(otlpSpan, &span); err != nil {
			return err
		}
		w.RawString(`{"span":`)
		if err := span.MarshalFastJSON(w); err != nil {
			return err
		}
		w.RawString("}\n")
	}

	// TODO(axw) we don't currently support sending arbitrary events
	// to Elastic APM Server. If/when we do, we should also transmit
	// otlpSpan.TimeEvents. When there's a convention specified for
	// error events, we could send those.

	return nil
}

func setTransactionProperties(
	otlpSpan pdata.Span,
	otlpLibrary pdata.InstrumentationLibrary,
	tx *model.Transaction, context *transactionContext,
) error {
	var (
		netHostName string
		netHostPort int
		netPeerIP   string
		netPeerPort int
	)

	otlpSpan.Attributes().ForEach(func(k string, v pdata.AttributeValue) {
		var storeTag bool
		switch k {
		// http.*
		case conventions.AttributeHTTPMethod:
			context.setHTTPMethod(v.StringVal())
		case conventions.AttributeHTTPURL:
			if err := context.setHTTPURL(v.StringVal()); err != nil {
				// Invalid http.url recorded by instrumentation,
				// record it as a label instead of in well-defined
				// url fields.
				storeTag = true
			}
		case conventions.AttributeHTTPTarget:
			if err := context.setHTTPURL(v.StringVal()); err != nil {
				// Invalid http.target recorded by instrumentation,
				// record it as a label instead of in well-defined
				// url fields.
				storeTag = true
			}
		case conventions.AttributeHTTPHost:
			context.setHTTPHost(v.StringVal())
		case conventions.AttributeHTTPScheme:
			context.setHTTPScheme(v.StringVal())
		case conventions.AttributeHTTPStatusCode:
			context.setHTTPStatusCode(int(v.IntVal()))
		case conventions.AttributeHTTPFlavor:
			context.setHTTPVersion(v.StringVal())
		case conventions.AttributeHTTPServerName:
			context.setHTTPHostname(v.StringVal())
		case conventions.AttributeHTTPClientIP:
			context.setHTTPRequestHeader("X-Forwarded-For", v.StringVal())
		case conventions.AttributeHTTPUserAgent:
			context.setHTTPRequestHeader("User-Agent", v.StringVal())
		case "http.remote_addr":
			// NOTE(axw) this is non-standard, sent by opentelemetry-go's othttp.
			// It's semantically equivalent to net.peer.ip+port. Standard attributes
			// take precedence.
			stringValue := v.StringVal()
			ip, port, err := net.SplitHostPort(stringValue)
			if err != nil {
				ip = stringValue
			}
			if net.ParseIP(ip) != nil {
				if netPeerIP == "" {
					netPeerIP = ip
				}
				if netPeerPort == 0 {
					netPeerPort, _ = strconv.Atoi(port)
				}
			}

		// net.*
		case conventions.AttributeNetPeerIP:
			netPeerIP = v.StringVal()
		case conventions.AttributeNetPeerPort:
			netPeerPort = int(v.IntVal())
		case conventions.AttributeNetHostName:
			netHostName = v.StringVal()
		case conventions.AttributeNetHostPort:
			netHostPort = int(v.IntVal())

		// other: record as a tag
		default:
			storeTag = true
		}
		if storeTag {
			context.model.Tags = append(context.model.Tags, model.IfaceMapItem{
				Key:   cleanLabelKey(k),
				Value: ifaceAttributeValue(v),
			})
		}
	})

	if !otlpLibrary.IsNil() {
		context.setFramework(otlpLibrary.Name(), otlpLibrary.Version())
	}

	statusCode := pdata.StatusCode(0) // Default span staus is "Ok"
	if status := otlpSpan.Status(); !status.IsNil() {
		statusCode = status.Code()
	}
	tx.Result = statusCode.String()

	tx.Type = "unknown"
	if context.model.Request != nil {
		tx.Type = "request"
		if context.model.Request.URL.Protocol == "" {
			// A bit presumptuous, but OpenTelemetry clients
			// are expected to send the scheme; this is just
			// a failsafe.
			context.model.Request.URL.Protocol = "http"
		}
		if context.model.Request.URL.Hostname == "" {
			context.model.Request.URL.Hostname = netHostName
		}
		if context.model.Request.URL.Port == "" && netHostPort > 0 {
			context.model.Request.URL.Port = strconv.Itoa(netHostPort)
		}
		if netPeerIP != "" {
			remoteAddr := netPeerIP
			if netPeerPort > 0 {
				remoteAddr = net.JoinHostPort(remoteAddr, strconv.Itoa(netPeerPort))
			}
			context.setHTTPRemoteAddr(remoteAddr)
		}
	}
	return nil
}

func setSpanProperties(otlpSpan pdata.Span, span *model.Span) error {
	var (
		context     spanContext
		netPeerName string
		netPeerIP   string
		netPeerPort int
	)

	otlpSpan.Attributes().ForEach(func(k string, v pdata.AttributeValue) {
		var storeTag bool
		switch k {
		// http.*
		case conventions.AttributeHTTPURL:
			if err := context.setHTTPURL(v.StringVal()); err != nil {
				// Invalid http.url recorded by instrumentation,
				// record it as a label instead of in well-defined
				// url fields.
				storeTag = true
			}
		case conventions.AttributeHTTPTarget:
			if err := context.setHTTPURL(v.StringVal()); err != nil {
				// Invalid http.target recorded by instrumentation,
				// record it as a label instead of in well-defined
				// url fields.
				storeTag = true
			}
		case conventions.AttributeHTTPHost:
			context.setHTTPHost(v.StringVal())
		case conventions.AttributeHTTPScheme:
			context.setHTTPScheme(v.StringVal())
		case conventions.AttributeHTTPStatusCode:
			context.setHTTPStatusCode(int(v.IntVal()))

		// net.*
		case conventions.AttributeNetPeerName:
			netPeerName = v.StringVal()
		case conventions.AttributeNetPeerIP:
			netPeerIP = v.StringVal()
		case conventions.AttributeNetPeerPort:
			netPeerPort = int(v.IntVal())

		// db.*
		case conventions.AttributeDBSystem:
			context.setDatabaseType(v.StringVal())
		case conventions.AttributeDBName:
			context.setDatabaseInstance(v.StringVal())
		case conventions.AttributeDBStatement:
			context.setDatabaseStatement(v.StringVal())
		case conventions.AttributeDBUser:
			context.setDatabaseUser(v.StringVal())

		// other: record as a tag
		default:
			storeTag = true
		}
		if storeTag {
			context.model.Tags = append(context.model.Tags, model.IfaceMapItem{
				Key:   cleanLabelKey(k),
				Value: ifaceAttributeValue(v),
			})
		}
	})

	destPort := netPeerPort
	destAddr := netPeerName
	if destAddr == "" {
		destAddr = netPeerIP
	}

	span.Type = "app"
	if context.model.HTTP != nil {
		span.Type = "external"
		span.Subtype = "http"
		if context.http.URL != nil {
			if context.http.URL.Scheme == "" {
				// A bit presumptuous, but OpenTelemetry clients
				// are expected to send the scheme; this is just
				// a failsafe.
				context.http.URL.Scheme = "http"
			}

			if context.http.URL.Host != "" {
				// Set destination.{address,port} from http.url.host.
				destAddr = context.http.URL.Hostname()
				if portString := context.http.URL.Port(); portString != "" {
					destPort, _ = strconv.Atoi(context.http.URL.Port())
				} else {
					destPort = schemeDefaultPort(context.http.URL.Scheme)
				}
			} else if destAddr != "" {
				// Set http.url.host from net.peer.*
				host := destAddr
				if destPort > 0 {
					port := strconv.Itoa(destPort)
					host = net.JoinHostPort(destAddr, port)
				}
				context.http.URL.Host = host
				if destPort == 0 {
					// Set destPort after setting http.url.host, so that
					// we don't include the default port there.
					destPort = schemeDefaultPort(context.http.URL.Scheme)
				}
			}

			// See https://github.com/elastic/apm/issues/180 for rules about setting
			// destination.service.* for external HTTP requests.
			destinationServiceURL := url.URL{Scheme: context.http.URL.Scheme, Host: context.http.URL.Host}
			destinationServiceResource := destinationServiceURL.Host
			if destPort != 0 && destPort == schemeDefaultPort(context.http.URL.Scheme) {
				if destinationServiceURL.Port() != "" {
					destinationServiceURL.Host = destinationServiceURL.Hostname()
				} else {
					destinationServiceResource = fmt.Sprintf("%s:%d", destinationServiceResource, destPort)
				}
			}
			context.setDestinationService(destinationServiceURL.String(), destinationServiceResource)
		}
	}
	if context.model.Database != nil {
		span.Type = "db"
		span.Subtype = context.model.Database.Type
		if span.Subtype != "" {
			// For database requests, we currently just identify the
			// destination service by db.system.
			context.setDestinationService(span.Subtype, span.Subtype)
		}
	}
	if destAddr != "" {
		context.setDestinationAddress(destAddr, destPort)
	}
	if context.model.Destination != nil && context.model.Destination.Service != nil {
		context.model.Destination.Service.Type = span.Type
	}
	span.Context = context.modelContext()
	return nil
}

type transactionContext struct {
	model            model.Context
	service          model.Service
	serviceFramework model.Framework
	request          model.Request
	requestSocket    model.RequestSocket
	response         model.Response
}

func (c *transactionContext) modelContext() *model.Context {
	switch {
	case c.model.Request != nil:
	case c.model.Response != nil:
	case c.model.Service != nil:
	case c.model.User != nil:
	case len(c.model.Tags) != 0:
	default:
		return nil
	}
	return &c.model
}

func (c *transactionContext) setFramework(name, version string) {
	if name == "" {
		return
	}
	c.serviceFramework.Name = truncate(name)
	c.serviceFramework.Version = truncate(version)
	c.service.Framework = &c.serviceFramework
	c.model.Service = &c.service
}

func (c *transactionContext) setHTTPMethod(method string) {
	c.request.Method = truncate(method)
	c.model.Request = &c.request
}

func (c *transactionContext) setHTTPScheme(scheme string) {
	c.request.URL.Protocol = truncate(scheme)
	c.model.Request = &c.request
}

func (c *transactionContext) setHTTPURL(httpURL string) error {
	u, err := url.Parse(httpURL)
	if err != nil {
		return err
	}
	// http.url is typically a relative URL, i.e. missing
	// the scheme and host. Don't override those parts of
	// the URL if they're empty, as they may be set by
	// other attributes.
	if u.Scheme != "" {
		c.request.URL.Protocol = truncate(u.Scheme)
	}
	if hostname := u.Hostname(); hostname != "" {
		c.request.URL.Hostname = truncate(hostname)
	}
	if port := u.Port(); port != "" {
		c.request.URL.Port = truncate(u.Port())
	}
	c.request.URL.Path = truncate(u.Path)
	c.request.URL.Search = truncate(u.RawQuery)
	c.request.URL.Hash = truncate(u.Fragment)
	c.model.Request = &c.request
	return nil
}

func (c *transactionContext) setHTTPHost(hostport string) {
	url := url.URL{Host: hostport}
	c.request.URL.Hostname = truncate(url.Hostname())
	c.request.URL.Port = truncate(url.Port())
	c.model.Request = &c.request
}

func (c *transactionContext) setHTTPHostname(hostname string) {
	c.request.URL.Hostname = truncate(hostname)
	c.model.Request = &c.request
}

func (c *transactionContext) setHTTPVersion(version string) {
	c.request.HTTPVersion = truncate(version)
	c.model.Request = &c.request
}

func (c *transactionContext) setHTTPRemoteAddr(remoteAddr string) {
	c.requestSocket.RemoteAddress = truncate(remoteAddr)
	c.request.Socket = &c.requestSocket
	c.model.Request = &c.request
}

func (c *transactionContext) setHTTPRequestHeader(k string, v ...string) {
	for i := range v {
		v[i] = truncate(v[i])
	}
	c.request.Headers = append(c.request.Headers, model.Header{Key: truncate(k), Values: v})
	c.model.Request = &c.request
}

func (c *transactionContext) setHTTPStatusCode(statusCode int) {
	c.response.StatusCode = statusCode
	c.model.Response = &c.response
}

type spanContext struct {
	model              model.SpanContext
	http               model.HTTPSpanContext
	httpURL            url.URL
	db                 model.DatabaseSpanContext
	destination        model.DestinationSpanContext
	destinationService model.DestinationServiceSpanContext
}

func (c *spanContext) modelContext() *model.SpanContext {
	switch {
	case c.model.HTTP != nil:
	case c.model.Database != nil:
	case c.model.Destination != nil:
	case len(c.model.Tags) != 0:
	default:
		return nil
	}
	return &c.model
}

func (c *spanContext) setHTTPStatusCode(statusCode int) {
	c.http.StatusCode = statusCode
	c.model.HTTP = &c.http
}

func (c *spanContext) setHTTPURL(httpURL string) error {
	u, err := url.Parse(httpURL)
	if err != nil {
		return err
	}
	// http.url may be a relative URL (http.target),
	// i.e. missing the scheme and host. Don't override
	// those parts of the URL if they're empty, as they
	// may be set by other attributes.
	if u.Scheme != "" {
		c.httpURL.Scheme = truncate(u.Scheme)
	}
	if u.Host != "" {
		c.httpURL.Host = truncate(u.Host)
	}
	c.httpURL.Path = truncate(u.Path)
	c.httpURL.RawQuery = truncate(u.RawQuery)
	c.httpURL.Fragment = truncate(u.Fragment)
	c.http.URL = &c.httpURL
	c.model.HTTP = &c.http
	return nil
}

func (c *spanContext) setHTTPScheme(httpScheme string) {
	c.httpURL.Scheme = truncate(httpScheme)
	c.http.URL = &c.httpURL
	c.model.HTTP = &c.http
}

func (c *spanContext) setHTTPHost(httpHost string) {
	c.httpURL.Host = truncate(httpHost)
	c.http.URL = &c.httpURL
	c.model.HTTP = &c.http
}

func (c *spanContext) setDatabaseType(dbType string) {
	c.db.Type = truncate(dbType)
	c.model.Database = &c.db
}

func (c *spanContext) setDatabaseInstance(dbInstance string) {
	c.db.Instance = truncate(dbInstance)
	c.model.Database = &c.db
}

func (c *spanContext) setDatabaseStatement(dbStatement string) {
	c.db.Statement = truncate(dbStatement)
	c.model.Database = &c.db
}

func (c *spanContext) setDatabaseUser(dbUser string) {
	c.db.User = truncate(dbUser)
	c.model.Database = &c.db
}

func (c *spanContext) setDestinationAddress(address string, port int) {
	c.destination.Address = truncate(address)
	c.destination.Port = port
	c.model.Destination = &c.destination
}

func (c *spanContext) setDestinationService(name, resource string) {
	c.destinationService.Name = truncate(name)
	c.destinationService.Resource = truncate(resource)
	c.destination.Service = &c.destinationService
	c.model.Destination = &c.destination
}

func schemeDefaultPort(scheme string) int {
	switch scheme {
	case "http":
		return 80
	case "https":
		return 443
	}
	return 0
}
