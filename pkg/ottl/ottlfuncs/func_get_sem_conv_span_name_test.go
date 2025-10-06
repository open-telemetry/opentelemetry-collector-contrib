package ottlfuncs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/attribute"
)

func TestSemConvSpanName(t *testing.T) {
	tests := []struct {
		name                   string
		currentSpanName        string // span name currently produced by the instrumentation library
		instrumentationLibrary string // instrumentation library used to produce the test case data
		kind                   ptrace.SpanKind
		addAttributes          func(pcommon.Map)
		want                   string
	}{
		{
			name:            "HTTP server span with both http.request.method and http.route",
			currentSpanName: "GET /users/123",
			kind:            ptrace.SpanKindServer,
			addAttributes: func(attrs pcommon.Map) {
				attrs.PutStr("http.request.method", "GET")
				attrs.PutStr("http.route", "/users/:id")
			},
			want: "GET /users/:id",
		},
		{
			name:            "HTTP server span with both deprecated http.method and http.route",
			currentSpanName: "GET /users/123",
			kind:            ptrace.SpanKindServer,
			addAttributes: func(attrs pcommon.Map) {
				attrs.PutStr("http.method", "GET")
				attrs.PutStr("http.route", "/users/:id")
			},
			want: "GET /users/:id",
		},
		{
			name:            "HTTP server span with just http.request.method",
			currentSpanName: "GET /users/123",
			kind:            ptrace.SpanKindServer,
			addAttributes: func(attrs pcommon.Map) {
				attrs.PutStr("http.request.method", "GET")
			},
			want: "GET",
		},
		{
			name:            "HTTP server span with just http.method",
			currentSpanName: "GET /users/123",
			kind:            ptrace.SpanKindServer,
			addAttributes: func(attrs pcommon.Map) {
				attrs.PutStr("http.method", "GET")
			},
			want: "GET",
		},
		{
			name:            "Fix for https://github.com/vercel/next.js/issues/54694",
			currentSpanName: "GET /app/workspaces/7?_rsc=hn5g2",
			kind:            ptrace.SpanKindServer,
			addAttributes: func(attrs pcommon.Map) {
				attrs.PutStr("http.method", "GET")
				attrs.PutStr("next.span_name", "GET /app/workspaces/7?_rsc=hn5g2")
				attrs.PutStr("next.span_type", "BaseServer.handleRequest")
				attrs.PutStr("http.target", "/app/workspaces/7?_rsc=hn5g2")
				attrs.PutStr("http.status", "200")
			},
			want: "GET",
		},
		{
			name:                   "Fix for OTelDemo problem caused by https://github.com/vercel/next.js/issues/54694",
			currentSpanName:        "GET /api/products/0PUK6V6EV0",
			instrumentationLibrary: "next.js:0.0.1",
			kind:                   ptrace.SpanKindServer,
			addAttributes: func(attrs pcommon.Map) {
				attrs.PutStr("http.method", "GET")
				attrs.PutStr("next.span_name", "GET /api/products/0PUK6V6EV0")
				attrs.PutStr("next.span_type", "BaseServer.handleRequest")
				attrs.PutBool("next.rsc", false)
				attrs.PutStr("http.target", "/api/products/0PUK6V6EV0")
				attrs.PutStr("http.status", "200")
			},
			want: "GET",
		},
		{
			name:                   "Fix for https://github.com/open-telemetry/opentelemetry-python-contrib/issues/1914",
			currentSpanName:        "GET /resource/9ea43cd7-bd77-494d-8fac-209c0dc7a438",
			instrumentationLibrary: "opentelemetry.instrumentation.pyramid.callbacks:",
			kind:                   ptrace.SpanKindServer,
			addAttributes: func(attrs pcommon.Map) {
				attrs.PutStr("http.method", "GET")
				attrs.PutStr("http.target", "/resource/9ea43cd7-bd77-494d-8fac-209c0dc7a438")
			},
			want: "GET",
		},

		// HTTP CLIENT SPANS
		{
			name:            "HTTP client span with both http.request.method and url.template",
			currentSpanName: "GET /users/123",
			kind:            ptrace.SpanKindClient,
			addAttributes: func(attrs pcommon.Map) {
				attrs.PutStr("http.request.method", "GET")
				attrs.PutStr("url.template", "/users/:id")
			},
			want: "GET /users/:id",
		},
		{
			name:            "HTTP client span with both deprecated http.method and url.template",
			currentSpanName: "GET /users/123",
			kind:            ptrace.SpanKindClient,
			addAttributes: func(attrs pcommon.Map) {
				attrs.PutStr("http.method", "GET")
				attrs.PutStr("url.template", "/users/:id")
			},
			want: "GET /users/:id",
		},
		{
			name:            "HTTP client span with just http.request.method",
			currentSpanName: "GET /users/123",
			kind:            ptrace.SpanKindClient,
			addAttributes: func(attrs pcommon.Map) {
				attrs.PutStr("http.request.method", "GET")
			},
			want: "GET",
		},
		{
			name:            "HTTP client span with just deprecated http.method",
			currentSpanName: "GET /users/123",
			kind:            ptrace.SpanKindClient,
			addAttributes: func(attrs pcommon.Map) {
				attrs.PutStr("http.method", "GET")
			},
			want: "GET",
		},
		// DB CLIENT SPANS
		{
			name:            "DB client span with db.system and db.operation.semConvSpanName - postgresql",
			currentSpanName: "INSERT webshop.orders",
			kind:            ptrace.SpanKindClient,
			addAttributes: func(attrs pcommon.Map) {
				attrs.PutStr("db.system.name", "postgresql")
				attrs.PutStr("db.namespace", "webshop")
				attrs.PutStr("db.operation.name", "INSERT")
				attrs.PutStr("db.collection.name", "orders")
				attrs.PutStr("db.query.text", "insert into orders (date_created,status) values (?,?)")
			},
			want: "INSERT webshop.orders",
		},
		{
			name:                   "OelDemo - cart - valkey/redis - HGET",
			currentSpanName:        "HGET",
			instrumentationLibrary: "OpenTelemetry.Instrumentation.StackExchangeRedis:1.11.0-beta.2",
			kind:                   ptrace.SpanKindClient,
			addAttributes: func(attrs pcommon.Map) {
				attrs.PutInt("db.redis.database_index", 0)
				attrs.PutStr("db.redis.flags", "None")
				attrs.PutStr("db.statement", "HGET 7175d9c6-9d66-11f0-b982-3258f881d4e5")
				attrs.PutStr("db.system", "redis")
				attrs.PutStr("server.address", "valkey-cart")
			},
			want: "(redis)",
		},
		{
			name:                   "DB client - OTel Demo - accounting",
			currentSpanName:        "otel",
			instrumentationLibrary: "NpgsqlLibrary:0.1.0",
			kind:                   ptrace.SpanKindClient,
			addAttributes: func(attrs pcommon.Map) {
				attrs.PutStr("db.system", "postgresql")
				attrs.PutInt("db.connection_id", 54)
				attrs.PutStr("db.connection_string", "Host=postgresql;Username=otelu;Database=otel")
				attrs.PutStr("db.name", "otel")
				attrs.PutStr("db.statement", `
INSERT INTO "order" (order_id)
VALUES (@p0);
INSERT INTO orderitem (order_id, product_id, item_cost_currency_code, item_cost_nanos, item_cost_units, quantity)
VALUES (@p1, @p2, @p3, @p4, @p5, @p6);
INSERT INTO shipping (shipping_tracking_id, city, country, order_id, shipping_cost_currency_code, shipping_cost_nanos, shipping_cost_units, state, street_address, zip_code)
VALUES (@p7, @p8, @p9, @p10, @p11, @p12, @p13, @p14, @p15, @p16);
`)
				attrs.PutStr("db.user", "otelu")
				attrs.PutStr("net.peer.name", "postgresql")
			},
			want: "otel",
		},

		// RPC - GRPC
		{
			name:                   "GRPC OTel Demo - checkout",
			currentSpanName:        "oteldemo.CartService/GetCart",
			instrumentationLibrary: "go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc:0.63.0",
			kind:                   ptrace.SpanKindClient,
			addAttributes: func(attrs pcommon.Map) {
				attrs.PutInt("rpc.grpc.status_code", 0)
				attrs.PutStr("rpc.grpc.method", "GetCart")
				attrs.PutStr("rpc.grpc.service", "oteldemo.CartService")
				attrs.PutStr("rpc.system", "grpc")
				attrs.PutStr("server.address", "127.18.0.18")
			},
			want: "oteldemo.CartService/GetCart",
		},
		{
			name:                   "GRPC OTel Demo - ad",
			currentSpanName:        "oteldemo.AdService/GetAds",
			instrumentationLibrary: "io.opentelemetry.grpc-1.6:2.20.0-alpha",
			kind:                   ptrace.SpanKindServer,
			addAttributes: func(attrs pcommon.Map) {
				attrs.PutInt("rpc.grpc.status_code", 0)
				attrs.PutStr("rpc.grpc.method", "GetAds")
				attrs.PutStr("rpc.grpc.service", "oteldemo.AdService")
				attrs.PutStr("rpc.system", "grpc")
				attrs.PutStr("server.address", "ad")
			},
			want: "oteldemo.AdService/GetAds",
		},

		// MESSAGING - KAFKA
		{
			name:                   "Messaging OTel Demo - accounting - ",
			currentSpanName:        "orders receive",
			instrumentationLibrary: "OpenTelemetry.AutoInstrumentation.Kafka",
			kind:                   ptrace.SpanKindConsumer,
			addAttributes: func(attrs pcommon.Map) {
				attrs.PutStr("messaging.client_id", "rdkafka#consumer-1")
				attrs.PutStr("messaging.destination.name", "orders")
				attrs.PutStr("messaging.kafka.consumer.group", "accounting")
				attrs.PutInt("messaging.kafka.destination.partition", 0)
				attrs.PutStr("messaging.operation", "receive")
				attrs.PutStr("messaging.system", "kafka")
			},
			want: "receive orders",
		},
		{
			name:                   "Messaging OTel Demo - fraud-detection - orders receive",
			currentSpanName:        "orders receive",
			instrumentationLibrary: "io.opentelemetry.kafka-clients-0.11:2.20.0-alpha",
			kind:                   ptrace.SpanKindConsumer,
			addAttributes: func(attrs pcommon.Map) {
				attrs.PutInt("messaging.batch.message_count", 4)
				attrs.PutStr("messaging.client_id", "consumer-fraud-detection-1")
				attrs.PutStr("messaging.destination.name", "orders")
				attrs.PutStr("messaging.kafka.consumer.group", "fraud-detection")
				attrs.PutStr("messaging.operation", "receive")
				attrs.PutStr("messaging.system", "kafka")
			},
			want: "receive orders",
		},
		{
			name:                   "Messaging OTel Demo - fraud-detection - orders process",
			currentSpanName:        "orders process",
			instrumentationLibrary: "io.opentelemetry.kafka-clients-0.11:2.20.0-alpha",
			kind:                   ptrace.SpanKindConsumer,
			addAttributes: func(attrs pcommon.Map) {
				attrs.PutInt("kafka.record.queue_time_ms", 3176788)
				attrs.PutStr("messaging.client_id", "consumer-fraud-detection-1")
				attrs.PutStr("messaging.destination.name", "orders")
				attrs.PutStr("messaging.destination.partition.id", "0")
				attrs.PutStr("messaging.kafka.consumer.group", "fraud-detection")
				attrs.PutInt("messaging.kafka.message.offset", 34)
				attrs.PutInt("messaging.message.body.size", 243)
				attrs.PutStr("messaging.operation", "process")
				attrs.PutStr("messaging.system", "kafka")
			},
			want: "process orders",
		},
		{
			name:                   "Messaging - OTel Demo - checkout - orders publish",
			currentSpanName:        "orders publish",
			instrumentationLibrary: "checkout",
			kind:                   ptrace.SpanKindProducer,
			addAttributes: func(attrs pcommon.Map) {
				attrs.PutStr("messaging.destination.name", "orders")
				attrs.PutInt("messaging.kafka.destination.partition", 0)
				attrs.PutInt("messaging.kafka.message.offset", 0)
				attrs.PutInt("messaging.kafka.producer.duration_ms", 0)
				attrs.PutBool("messaging.kafka.producer.success", true)
				attrs.PutStr("messaging.operation", "publish")
				attrs.PutStr("messaging.system", "kafka")
			},
			want: "publish orders",
		},

		// MESSAGING - RABBIT MQ
		{
			name:                   "Messaging consumer span with messaging.system, messaging.operation, and messaging.destination.name",
			currentSpanName:        "process ecommerce-exchange",
			instrumentationLibrary: "io.opentelemetry.rabbitmq-2.7:2.20.0-alpha",
			kind:                   ptrace.SpanKindConsumer,
			addAttributes: func(attrs pcommon.Map) {
				attrs.PutStr("messaging.system", "rabbitmq")
				attrs.PutStr("messaging.destination.name", "ecommerce-exchange")
				attrs.PutStr("messaging.operation", "process")
				attrs.PutStr("messaging.rabbitmq.destination.routing_key", "queue.order")
			},
			want: "process ecommerce-exchange",
		},
		{
			name:                   "Messaging consumer span with messaging.system, messaging.operation, and messaging.destination.name",
			currentSpanName:        "process queue.order",
			instrumentationLibrary: "io.opentelemetry.spring-rabbit-1.0:2.20-alpha",
			kind:                   ptrace.SpanKindConsumer,
			addAttributes: func(attrs pcommon.Map) {
				attrs.PutStr("messaging.system", "rabbitmq")
				attrs.PutStr("messaging.destination.name", "queue.order")
				attrs.PutStr("messaging.operation", "process")
			},
			want: "process queue.order",
		},
		{
			name:                   "Messaging producer span with messaging.system, messaging.operation, and messaging.destination.name",
			currentSpanName:        "publish ecommerce-exchange",
			instrumentationLibrary: "io.opentelemetry.rabbitmq-2.7:2.20.0-alpha",
			kind:                   ptrace.SpanKindProducer,
			addAttributes: func(attrs pcommon.Map) {
				attrs.PutStr("messaging.system", "rabbitmq")
				attrs.PutStr("messaging.destination.name", "ecommerce-exchange")
				attrs.PutStr("messaging.operation", "publish")
				attrs.PutStr("messaging.rabbitmq.destination.routing_key", "queue.order")
			},
			want: "publish ecommerce-exchange",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			span := ptrace.NewSpan()
			span.SetName(tt.currentSpanName)
			span.SetKind(tt.kind)
			tt.addAttributes(span.Attributes())
			assert.Equal(t, tt.want, SemConvSpanName(span))
		})
	}
}

func Test_attributeValue(t *testing.T) {
	tests := []struct {
		name                    string
		spanName                string
		kind                    ptrace.SpanKind
		attributeName           attribute.Key
		deprecatedAttributeName string
		addAttributes           func(pcommon.Map)
		wantVal                 string
		wantOk                  bool
	}{
		{
			name:                    "HTTP server span with both http.request.method and http.route",
			spanName:                "GET /users/:id",
			kind:                    ptrace.SpanKindServer,
			attributeName:           "http.request.method",
			deprecatedAttributeName: "http.method",
			addAttributes: func(attrs pcommon.Map) {
				attrs.PutStr("http.request.method", "GET")
				attrs.PutStr("http.route", "/users/:id")
			},
			wantVal: "GET",
			wantOk:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.spanName, func(t *testing.T) {
			span := ptrace.NewSpan()
			span.SetName(tt.spanName)
			span.SetKind(tt.kind)
			tt.addAttributes(span.Attributes())

			gotVal, gotOk := attributeValue(span, tt.attributeName, tt.deprecatedAttributeName)
			assert.Equalf(t, tt.wantOk, gotOk, "attributeValue(%v, %v)", tt.attributeName, tt.deprecatedAttributeName)
			assert.Equalf(t, tt.wantVal, gotVal.AsString(), "attributeValue(%v, %v)", tt.attributeName, tt.deprecatedAttributeName)
		})
	}
}

func Test_messagingDestination(t *testing.T) {
	tests := []struct {
		name           string
		spanName       string
		kind           ptrace.SpanKind
		attributeNames []string
		addAttributes  func(pcommon.Map)
		want           string
	}{
		{
			name:     "Templated queue",
			spanName: "send /customers/{customerId}",
			kind:     ptrace.SpanKindProducer,
			addAttributes: func(attrs pcommon.Map) {
				attrs.PutStr("messaging.operation.name", "send")
				attrs.PutStr("messaging.destination.template", "/customers/{customerId}")
				attrs.PutStr("messaging.destination.name", "/customers/123456")
			},
			want: "/customers/{customerId}",
		},
		{
			name:     "Temporary queue",
			spanName: "send (temporary)",
			kind:     ptrace.SpanKindProducer,
			addAttributes: func(attrs pcommon.Map) {
				attrs.PutStr("messaging.operation.name", "send")
				attrs.PutStr("messaging.destination.name", "amq.gen-JzTY20BRgKO-HjmUJj0wLg")
				attrs.PutBool("messaging.destination.temporary", true)
			},
			want: "(temporary)",
		},
		{
			name:     "Anonymous queue",
			spanName: "send (anonymous)",
			kind:     ptrace.SpanKindProducer,
			addAttributes: func(attrs pcommon.Map) {
				attrs.PutStr("messaging.operation.name", "send")
				attrs.PutStr("messaging.destination.name", "amq.gen-3j8Jks9dTQWm1y2zLx0r5w")
				attrs.PutBool("messaging.destination.anonymous", true)
				// anonymous queues should also have messaging.destination.temporary:true
				// but we omit it for the unit test to test the robustness of the logic
			},
			want: "(anonymous)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			span := ptrace.NewSpan()
			span.SetName(tt.spanName)
			span.SetKind(tt.kind)
			tt.addAttributes(span.Attributes())

			got := messagingDestination(span)
			assert.Equalf(t, tt.want, got, "messagingDestination(%v)", span)
		})
	}
}

func Test_rpcSpanName(t *testing.T) {
	tests := []struct {
		name                   string
		spanName               string
		instrumentationLibrary string
		kind                   ptrace.SpanKind
		attributeNames         []string
		addAttributes          func(pcommon.Map)
		want                   string
	}{
		{
			name:                   "GRPC OTel Demo - checkout",
			spanName:               "oteldemo.CartService/GetCart",
			instrumentationLibrary: "go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc:0.63.0",
			kind:                   ptrace.SpanKindClient,
			addAttributes: func(attrs pcommon.Map) {
				attrs.PutInt("rpc.grpc.status_code", 0)
				attrs.PutStr("rpc.grpc.method", "GetCart")
				attrs.PutStr("rpc.grpc.service", "oteldemo.CartService")
				attrs.PutStr("rpc.system", "grpc")
				attrs.PutStr("server.address", "127.18.0.18")
			},
			want: "oteldemo.CartService/GetCart",
		},
		{
			name:                   "GRPC OTel Demo - ad",
			spanName:               "oteldemo.AdService/GetAds",
			instrumentationLibrary: "io.opentelemetry.grpc-1.6:2.20.0-alpha",
			kind:                   ptrace.SpanKindServer,
			addAttributes: func(attrs pcommon.Map) {
				attrs.PutInt("rpc.grpc.status_code", 0)
				attrs.PutStr("rpc.grpc.method", "GetAds")
				attrs.PutStr("rpc.grpc.service", "oteldemo.AdService")
				attrs.PutStr("rpc.system", "grpc")
				attrs.PutStr("server.address", "ad")
			},
			want: "oteldemo.AdService/GetAds",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			span := ptrace.NewSpan()
			span.SetName(tt.spanName)
			span.SetKind(tt.kind)
			tt.addAttributes(span.Attributes())
			got := rpcSpanName(span)
			assert.Equalf(t, tt.want, got, "getRPCSpanName(%v)", span)
		})
	}
}
