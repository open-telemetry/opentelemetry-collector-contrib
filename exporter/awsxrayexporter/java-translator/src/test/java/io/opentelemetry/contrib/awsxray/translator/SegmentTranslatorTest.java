// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package io.opentelemetry.contrib.awsxray.translator;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.fasterxml.jackson.databind.ObjectMapper;
import io.opentelemetry.api.common.AttributeKey;
import io.opentelemetry.api.common.Attributes;
import io.opentelemetry.api.trace.SpanContext;
import io.opentelemetry.api.trace.SpanKind;
import io.opentelemetry.api.trace.TraceFlags;
import io.opentelemetry.api.trace.TraceState;
import io.opentelemetry.contrib.awsxray.translator.model.Segment;
import io.opentelemetry.sdk.resources.Resource;
import io.opentelemetry.sdk.testing.trace.TestSpanData;
import io.opentelemetry.sdk.trace.data.SpanData;
import io.opentelemetry.sdk.trace.data.StatusData;
import java.time.Instant;
import java.util.List;
import java.util.Map;
import org.junit.jupiter.api.Test;

@SuppressWarnings("unchecked")
class SegmentTranslatorTest {

    private static final ObjectMapper MAPPER = new ObjectMapper();

    private final SegmentTranslator translator = SegmentTranslator.builder()
            .skipTimestampValidation(true)
            .build();

    private static String makeTraceId() {
        long now = Instant.now().getEpochSecond();
        return String.format("%08x", now) + "a006649127e371903a2de979";
    }

    @Test
    void testServerSpanTranslation() throws Exception {
        String traceId = makeTraceId();
        String spanId = "1234567890abcdef";
        long now = Instant.now().getEpochSecond();

        SpanData span = TestSpanData.builder()
                .setName("GET /api/users")
                .setKind(SpanKind.SERVER)
                .setSpanContext(SpanContext.create(traceId, spanId, TraceFlags.getSampled(), TraceState.getDefault()))
                .setStartEpochNanos(now * 1_000_000_000L)
                .setEndEpochNanos((now + 1) * 1_000_000_000L)
                .setStatus(StatusData.ok())
                .setHasEnded(true)
                .setResource(Resource.create(Attributes.of(
                        AttributeKey.stringKey("service.name"), "test-service",
                        AttributeKey.stringKey("cloud.provider"), "aws",
                        AttributeKey.stringKey("cloud.platform"), "aws_ec2",
                        AttributeKey.stringKey("host.id"), "i-1234567890"
                )))
                .setAttributes(Attributes.of(
                        AttributeKey.stringKey("http.method"), "GET",
                        AttributeKey.longKey("http.status_code"), 200L,
                        AttributeKey.stringKey("http.url"), "https://example.com/api/users"
                ))
                .build();

        List<String> documents = translator.makeSegmentDocuments(span);
        assertThat(documents).hasSize(1);

        Map<String, Object> doc = MAPPER.readValue(documents.get(0), Map.class);
        assertThat(doc.get("name")).isEqualTo("test-service");
        assertThat(doc.get("id")).isEqualTo(spanId);
        assertThat(doc.get("fault")).isEqualTo(false);
        assertThat(doc.get("error")).isEqualTo(false);

        Map<String, Object> http = (Map<String, Object>) doc.get("http");
        assertThat(http).isNotNull();
        Map<String, Object> request = (Map<String, Object>) http.get("request");
        assertThat(request.get("method")).isEqualTo("GET");
        assertThat(request.get("url")).isEqualTo("https://example.com/api/users");

        Map<String, Object> aws = (Map<String, Object>) doc.get("aws");
        assertThat(aws).isNotNull();
        Map<String, Object> ec2 = (Map<String, Object>) aws.get("ec2");
        assertThat(ec2).isNotNull();
        assertThat(ec2.get("instance_id")).isEqualTo("i-1234567890");
    }

    @Test
    void testClientSpanWithNamespace() throws Exception {
        String traceId = makeTraceId();
        String spanId = "abcdef1234567890";
        String parentSpanId = "1234567890abcdef";
        long now = Instant.now().getEpochSecond();

        SpanData span = TestSpanData.builder()
                .setName("DynamoDB.PutItem")
                .setKind(SpanKind.CLIENT)
                .setSpanContext(SpanContext.create(traceId, spanId, TraceFlags.getSampled(), TraceState.getDefault()))
                .setParentSpanContext(SpanContext.create(traceId, parentSpanId, TraceFlags.getSampled(), TraceState.getDefault()))
                .setStartEpochNanos(now * 1_000_000_000L)
                .setEndEpochNanos((now + 1) * 1_000_000_000L)
                .setStatus(StatusData.ok())
                .setHasEnded(true)
                .setResource(Resource.create(Attributes.of(
                        AttributeKey.stringKey("service.name"), "my-service"
                )))
                .setAttributes(Attributes.of(
                        AttributeKey.stringKey("rpc.system"), "aws-api",
                        AttributeKey.stringKey("rpc.method"), "PutItem",
                        AttributeKey.stringKey("aws.service"), "DynamoDB"
                ))
                .build();

        Segment segment = translator.makeSegment(span);
        assertThat(segment.getName()).isEqualTo("DynamoDB");
        assertThat(segment.getNamespace()).isEqualTo("aws");
        assertThat(segment.getType()).isEqualTo("subsegment");
    }

    @Test
    void testErrorSpan() throws Exception {
        String traceId = makeTraceId();
        String spanId = "fedcba0987654321";
        long now = Instant.now().getEpochSecond();

        SpanData span = TestSpanData.builder()
                .setName("GET /api/error")
                .setKind(SpanKind.SERVER)
                .setSpanContext(SpanContext.create(traceId, spanId, TraceFlags.getSampled(), TraceState.getDefault()))
                .setStartEpochNanos(now * 1_000_000_000L)
                .setEndEpochNanos((now + 1) * 1_000_000_000L)
                .setStatus(StatusData.error())
                .setHasEnded(true)
                .setResource(Resource.create(Attributes.of(
                        AttributeKey.stringKey("service.name"), "test-service"
                )))
                .setAttributes(Attributes.of(
                        AttributeKey.longKey("http.status_code"), 500L
                ))
                .build();

        Segment segment = translator.makeSegment(span);
        assertThat(segment.getFault()).isTrue();
        assertThat(segment.getError()).isFalse();
    }

    @Test
    void testThrottleSpan() throws Exception {
        String traceId = makeTraceId();
        String spanId = "1111111111111111";
        long now = Instant.now().getEpochSecond();

        SpanData span = TestSpanData.builder()
                .setName("GET /api/throttled")
                .setKind(SpanKind.SERVER)
                .setSpanContext(SpanContext.create(traceId, spanId, TraceFlags.getSampled(), TraceState.getDefault()))
                .setStartEpochNanos(now * 1_000_000_000L)
                .setEndEpochNanos((now + 1) * 1_000_000_000L)
                .setStatus(StatusData.error())
                .setHasEnded(true)
                .setResource(Resource.create(Attributes.of(
                        AttributeKey.stringKey("service.name"), "test-service"
                )))
                .setAttributes(Attributes.of(
                        AttributeKey.longKey("http.status_code"), 429L
                ))
                .build();

        Segment segment = translator.makeSegment(span);
        assertThat(segment.getThrottle()).isTrue();
        assertThat(segment.getError()).isTrue();
        assertThat(segment.getFault()).isFalse();
    }

    @Test
    void testTraceIdConversion() throws Exception {
        SegmentTranslator t = SegmentTranslator.builder().skipTimestampValidation(true).build();
        String result = t.convertToAmazonTraceId("5f84c58410a06c0c00000000a0000001");
        assertThat(result).isEqualTo("1-5f84c584-10a06c0c00000000a0000001");
    }

    @Test
    void testTraceIdValidationFailure() {
        SegmentTranslator strict = SegmentTranslator.builder().skipTimestampValidation(false).build();
        assertThatThrownBy(() -> strict.convertToAmazonTraceId("0000000010a06c0c00000000a0000001"))
                .isInstanceOf(TranslationException.class)
                .hasMessageContaining("Invalid xray traceid");
    }

    @Test
    void testFixSegmentName() {
        assertThat(SegmentTranslator.fixSegmentName("valid-name_123")).isEqualTo("valid-name_123");
        assertThat(SegmentTranslator.fixSegmentName("invalid<>name")).isEqualTo("invalidname");
        assertThat(SegmentTranslator.fixSegmentName("")).isEqualTo("span");
    }

    @Test
    void testFixAnnotationKey() {
        assertThat(SegmentTranslator.fixAnnotationKey("simple")).isEqualTo("simple");
        // With allowDot=true (default), dots are preserved
        assertThat(SegmentTranslator.fixAnnotationKey("my.key")).isEqualTo("my.key");
        assertThat(SegmentTranslator.fixAnnotationKey("key-with-dashes")).isEqualTo("key_with_dashes");
        // With allowDot=false, dots are converted to underscores
        assertThat(SegmentTranslator.fixAnnotationKey("my.key", false)).isEqualTo("my_key");
        assertThat(SegmentTranslator.fixAnnotationKey("my.key", true)).isEqualTo("my.key");
    }

    @Test
    void testDetermineAwsOrigin() {
        Attributes ec2 = Attributes.of(
                AttributeKey.stringKey("cloud.provider"), "aws",
                AttributeKey.stringKey("cloud.platform"), "aws_ec2"
        );
        assertThat(SegmentTranslator.determineAwsOrigin(ec2)).isEqualTo("AWS::EC2::Instance");

        Attributes eks = Attributes.of(
                AttributeKey.stringKey("cloud.provider"), "aws",
                AttributeKey.stringKey("cloud.platform"), "aws_eks"
        );
        assertThat(SegmentTranslator.determineAwsOrigin(eks)).isEqualTo("AWS::EKS::Container");

        Attributes gcp = Attributes.of(
                AttributeKey.stringKey("cloud.provider"), "gcp",
                AttributeKey.stringKey("cloud.platform"), "gcp_compute_engine"
        );
        assertThat(SegmentTranslator.determineAwsOrigin(gcp)).isEqualTo("");
    }

    @Test
    void testSqlSpan() throws Exception {
        String traceId = makeTraceId();
        String spanId = "2222222222222222";
        String parentSpanId = "3333333333333333";
        long now = Instant.now().getEpochSecond();

        SpanData span = TestSpanData.builder()
                .setName("SELECT users")
                .setKind(SpanKind.CLIENT)
                .setSpanContext(SpanContext.create(traceId, spanId, TraceFlags.getSampled(), TraceState.getDefault()))
                .setParentSpanContext(SpanContext.create(traceId, parentSpanId, TraceFlags.getSampled(), TraceState.getDefault()))
                .setStartEpochNanos(now * 1_000_000_000L)
                .setEndEpochNanos((now + 1) * 1_000_000_000L)
                .setStatus(StatusData.ok())
                .setHasEnded(true)
                .setResource(Resource.create(Attributes.empty()))
                .setAttributes(Attributes.of(
                        AttributeKey.stringKey("db.system"), "postgresql",
                        AttributeKey.stringKey("db.name"), "mydb",
                        AttributeKey.stringKey("db.statement"), "SELECT * FROM users",
                        AttributeKey.stringKey("db.user"), "admin"
                ))
                .build();

        Segment segment = translator.makeSegment(span);
        assertThat(segment.getSql()).isNotNull();
        assertThat(segment.getSql().getDatabaseType()).isEqualTo("postgresql");
        assertThat(segment.getSql().getUser()).isEqualTo("admin");
        assertThat(segment.getSql().getSanitizedQuery()).isEqualTo("SELECT * FROM users");
        assertThat(segment.getName()).isEqualTo("mydb");
    }
}
