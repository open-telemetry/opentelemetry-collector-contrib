// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package io.opentelemetry.contrib.awsxray.translator;

import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;
import io.opentelemetry.api.common.AttributeKey;
import io.opentelemetry.api.common.Attributes;
import io.opentelemetry.api.trace.SpanKind;
import io.opentelemetry.contrib.awsxray.translator.model.AwsData;
import io.opentelemetry.contrib.awsxray.translator.model.CauseData;
import io.opentelemetry.contrib.awsxray.translator.model.HttpData;
import io.opentelemetry.contrib.awsxray.translator.model.Segment;
import io.opentelemetry.contrib.awsxray.translator.model.ServiceData;
import io.opentelemetry.contrib.awsxray.translator.model.SpanLinkData;
import io.opentelemetry.contrib.awsxray.translator.model.SqlData;
import io.opentelemetry.sdk.resources.Resource;
import io.opentelemetry.sdk.trace.data.LinkData;
import io.opentelemetry.sdk.trace.data.SpanData;
import java.net.URI;
import java.nio.ByteBuffer;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Collections;
import java.util.HashMap;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.regex.Pattern;

import static io.opentelemetry.contrib.awsxray.translator.AttributeConstants.*;

/**
 * Translates OpenTelemetry spans into AWS X-Ray segment documents.
 * This is the Java equivalent of the Go translator in the opentelemetry-collector-contrib project.
 */
public class SegmentTranslator {

    private static final ObjectMapper OBJECT_MAPPER = new ObjectMapper()
            .setSerializationInclusion(JsonInclude.Include.NON_NULL);

    private static final Pattern INVALID_SPAN_CHARACTERS = Pattern.compile("[^ 0-9\\p{L}N_.:/%&#=+\\-@]");
    private static final int MAX_SEGMENT_NAME_LENGTH = 200;
    private static final String DEFAULT_SEGMENT_NAME = "span";
    private static final String DEFAULT_METADATA_NAMESPACE = "default";

    // AWS X-Ray origin values
    public static final String ORIGIN_EC2 = "AWS::EC2::Instance";
    public static final String ORIGIN_ECS = "AWS::ECS::Container";
    public static final String ORIGIN_ECS_EC2 = "AWS::ECS::EC2";
    public static final String ORIGIN_ECS_FARGATE = "AWS::ECS::Fargate";
    public static final String ORIGIN_EB = "AWS::ElasticBeanstalk::Environment";
    public static final String ORIGIN_EKS = "AWS::EKS::Container";
    public static final String ORIGIN_APP_RUNNER = "AWS::AppRunner::Service";

    private static final Set<String> REMOVE_ANNOTATIONS_FROM_SERVICE_SEGMENT = Set.of(
            AWS_REMOTE_SERVICE, AWS_REMOTE_OPERATION, REMOTE_TARGET, K8S_REMOTE_NAMESPACE
    );

    private final List<String> indexedAttrs;
    private final boolean indexAllAttrs;
    private final List<String> logGroupNames;
    private final boolean skipTimestampValidation;
    private final boolean allowDotInAnnotationKeys;

    public SegmentTranslator() {
        this(Collections.emptyList(), false, Collections.emptyList(), false, true);
    }

    public SegmentTranslator(List<String> indexedAttrs, boolean indexAllAttrs,
                             List<String> logGroupNames, boolean skipTimestampValidation,
                             boolean allowDotInAnnotationKeys) {
        this.indexedAttrs = indexedAttrs;
        this.indexAllAttrs = indexAllAttrs;
        this.logGroupNames = logGroupNames;
        this.skipTimestampValidation = skipTimestampValidation;
        this.allowDotInAnnotationKeys = allowDotInAnnotationKeys;
    }

    /**
     * Converts an OpenTelemetry span to one or more X-Ray segment JSON documents.
     */
    public List<String> makeSegmentDocuments(SpanData span) throws TranslationException {
        List<Segment> segments = makeSegmentsFromSpan(span);
        List<String> documents = new ArrayList<>();
        for (Segment segment : segments) {
            documents.add(makeDocumentFromSegment(segment));
        }
        return documents;
    }

    /**
     * Converts a Segment object to a JSON string.
     */
    public String makeDocumentFromSegment(Segment segment) throws TranslationException {
        try {
            return OBJECT_MAPPER.writeValueAsString(segment);
        } catch (JsonProcessingException e) {
            throw new TranslationException("Failed to serialize segment to JSON", e);
        }
    }

    /**
     * Creates one or more segments from a span.
     */
    public List<Segment> makeSegmentsFromSpan(SpanData span) throws TranslationException {
        if (!isLocalRoot(span)) {
            return makeNonLocalRootSegment(span);
        }
        if (!isLocalRootSpanADependencySpan(span)) {
            return makeServiceSegmentForLocalRootSpanWithoutDependency(span);
        }
        return makeServiceSegmentAndDependencySubsegment(span);
    }

    private List<Segment> makeNonLocalRootSegment(SpanData span) throws TranslationException {
        Segment segment = makeSegment(span);
        addNamespaceToSubsegmentWithRemoteService(span, segment);
        return List.of(segment);
    }

    private List<Segment> makeServiceSegmentForLocalRootSpanWithoutDependency(SpanData span) throws TranslationException {
        Segment segment = makeSegment(span);
        segment.setType(null);
        segment.setNamespace(null);
        return List.of(segment);
    }

    private List<Segment> makeServiceSegmentAndDependencySubsegment(SpanData span) throws TranslationException {
        String serviceSegmentId = CauseTranslator.generateSegmentId();
        List<Segment> segments = new ArrayList<>();

        // Dependency subsegment
        Segment dependencySubsegment = makeSegment(span);
        dependencySubsegment.setParentId(serviceSegmentId);
        dependencySubsegment.setType("subsegment");
        if (dependencySubsegment.getNamespace() == null) {
            dependencySubsegment.setNamespace("remote");
        }
        if (span.getKind() == SpanKind.CONSUMER) {
            dependencySubsegment.setLinks(null);
        }
        Object remoteService = span.getAttributes().get(AttributeKey.stringKey(AWS_REMOTE_SERVICE));
        if (remoteService != null) {
            String subsegmentName = trimAwsSdkPrefix(String.valueOf(remoteService), span);
            dependencySubsegment.setName(subsegmentName);
        }
        segments.add(dependencySubsegment);

        // Service segment
        Segment serviceSegment = makeSegment(span);
        serviceSegment.setId(serviceSegmentId);
        Object localService = span.getAttributes().get(AttributeKey.stringKey(AWS_LOCAL_SERVICE));
        if (localService != null) {
            serviceSegment.setName(String.valueOf(localService));
        }
        serviceSegment.setHttp(null);
        serviceSegment.setType(null);
        serviceSegment.setNamespace(null);
        if (span.getKind() != SpanKind.CONSUMER) {
            serviceSegment.setLinks(null);
        }
        // Clear AWS subsegment-specific fields
        if (serviceSegment.getAws() != null) {
            serviceSegment.getAws().setOperation(null);
            serviceSegment.getAws().setAccountId(null);
            serviceSegment.getAws().setRemoteRegion(null);
            serviceSegment.getAws().setRequestId(null);
            serviceSegment.getAws().setQueueUrl(null);
            serviceSegment.getAws().setTableName(null);
            serviceSegment.getAws().setTableNames(null);
        }
        // Remove remote-specific annotations from the service segment
        if (serviceSegment.getAnnotations() != null) {
            for (String key : REMOVE_ANNOTATIONS_FROM_SERVICE_SEGMENT) {
                serviceSegment.getAnnotations().remove(fixAnnotationKey(key));
            }
            if (serviceSegment.getAnnotations().isEmpty()) {
                serviceSegment.setAnnotations(null);
            }
        }
        // Filter metadata — keep only otel.resource.* entries
        if (serviceSegment.getMetadata() != null) {
            for (Map<String, Object> metaDataEntry : serviceSegment.getMetadata().values()) {
                metaDataEntry.entrySet().removeIf(e -> !e.getKey().startsWith("otel.resource."));
            }
            serviceSegment.getMetadata().entrySet().removeIf(e -> e.getValue().isEmpty());
            if (serviceSegment.getMetadata().isEmpty()) {
                serviceSegment.setMetadata(null);
            }
        }
        segments.add(serviceSegment);

        return segments;
    }

    /**
     * Converts an OpenTelemetry Span to an X-Ray Segment.
     */
    public Segment makeSegment(SpanData span) throws TranslationException {
        String segmentType = null;
        boolean storeResource = true;

        if (span.getKind() != SpanKind.SERVER
                && span.getKind() != SpanKind.CONSUMER
                && span.getParentSpanContext().isValid()) {
            segmentType = "subsegment";
            storeResource = false;
        }

        String traceId = convertToAmazonTraceId(span.getTraceId());
        Resource resource = span.getResource();
        Attributes resourceAttributes = resource.getAttributes();

        double startTime = span.getStartEpochNanos() / 1_000_000_000.0;
        double endTime = span.getEndEpochNanos() / 1_000_000_000.0;
        Boolean inProgress = null;

        Object inProgressAttr = span.getAttributes().get(AttributeKey.booleanKey(AWS_XRAY_INPROGRESS));
        if (Boolean.TRUE.equals(inProgressAttr)) {
            inProgress = true;
        }

        // HTTP
        HttpTranslator.HttpResult httpResult = HttpTranslator.makeHttp(span);
        // Remove aws.xray.inprogress from filtered attributes so it doesn't leak into metadata
        httpResult.filteredAttributes.remove(AWS_XRAY_INPROGRESS);
        HttpData httpData = httpResult.httpData;

        // Cause
        CauseTranslator.CauseResult causeResult = CauseTranslator.makeCause(span, httpResult.filteredAttributes, resourceAttributes);
        boolean isError = causeResult.isError;
        boolean isFault = causeResult.isFault;
        boolean isThrottle = causeResult.isThrottle;
        CauseData causeData = causeResult.causeData;

        // Origin
        String origin = determineAwsOrigin(resourceAttributes);

        // AWS
        AwsTranslator.AwsResult awsResult = AwsTranslator.makeAws(causeResult.filteredAttributes, resourceAttributes, logGroupNames);
        AwsData awsData = awsResult.awsData;

        // Service
        ServiceData serviceData = makeService(resourceAttributes);

        // SQL
        SqlTranslator.SqlResult sqlResult = SqlTranslator.makeSql(span, awsResult.filteredAttributes);
        SqlData sqlData = sqlResult.sqlData;

        // Annotations and metadata
        Map<String, Object> additionalAttrs = addSpecialAttributes(sqlResult.filteredAttributes, span.getAttributes());
        XRayAttributesResult xrayAttrs = makeXRayAttributes(additionalAttrs, resourceAttributes, storeResource);
        String user = xrayAttrs.user;
        Map<String, Object> annotations = xrayAttrs.annotations;
        Map<String, Map<String, Object>> metadata = xrayAttrs.metadata;

        // Span links
        List<SpanLinkData> spanLinks = makeSpanLinks(span.getLinks());

        // Determine name and namespace
        String name = "";
        String namespace = "";

        Attributes spanAttributes = span.getAttributes();

        if (span.getKind() == SpanKind.SERVER) {
            Object localServiceName = spanAttributes.get(AttributeKey.stringKey(AWS_LOCAL_SERVICE));
            if (localServiceName != null) {
                name = String.valueOf(localServiceName);
            }
        }

        Object awsSpanKindVal = spanAttributes.get(AttributeKey.stringKey(AWS_SPAN_KIND));
        if (span.getKind() == SpanKind.INTERNAL && LOCAL_ROOT.equals(awsSpanKindVal)) {
            Object localServiceName = spanAttributes.get(AttributeKey.stringKey(AWS_LOCAL_SERVICE));
            if (localServiceName != null) {
                name = String.valueOf(localServiceName);
            }
        }

        if (span.getKind() == SpanKind.CLIENT || span.getKind() == SpanKind.PRODUCER || span.getKind() == SpanKind.CONSUMER) {
            Object remoteServiceName = spanAttributes.get(AttributeKey.stringKey(AWS_REMOTE_SERVICE));
            if (remoteServiceName != null) {
                name = trimAwsSdkPrefix(String.valueOf(remoteServiceName), span);
            }
        }

        if (name.isEmpty()) {
            Object peerService = spanAttributes.get(AttributeKey.stringKey(PEER_SERVICE));
            if (peerService != null) {
                name = String.valueOf(peerService);
            }
        }

        if (namespace.isEmpty()) {
            if (isAwsSdkSpan(span)) {
                namespace = CLOUD_PROVIDER_AWS;
            }
        }

        if (name.isEmpty()) {
            Object awsService = spanAttributes.get(AttributeKey.stringKey(AWS_SERVICE));
            if (awsService != null) {
                name = String.valueOf(awsService);
                if (namespace.isEmpty()) {
                    namespace = CLOUD_PROVIDER_AWS;
                }
            }
        }

        if (name.isEmpty()) {
            Object dbInstance = spanAttributes.get(AttributeKey.stringKey(DB_NAME));
            if (dbInstance != null) {
                name = String.valueOf(dbInstance);
                Object dbUrl = spanAttributes.get(AttributeKey.stringKey(DB_CONNECTION_STRING));
                if (dbUrl != null) {
                    String dbUrlStr = String.valueOf(dbUrl);
                    if (dbUrlStr.startsWith("jdbc:")) {
                        dbUrlStr = dbUrlStr.substring(5);
                    }
                    try {
                        URI uri = URI.create(dbUrlStr);
                        if (uri.getHost() != null && !uri.getHost().isEmpty()) {
                            name += "@" + uri.getHost();
                        }
                    } catch (Exception ignored) {}
                }
            }
        }

        if (name.isEmpty() && span.getKind() == SpanKind.SERVER) {
            Object serviceName = resourceAttributes.get(AttributeKey.stringKey(SERVICE_NAME));
            if (serviceName != null) {
                name = String.valueOf(serviceName);
            }
        }

        if (name.isEmpty()) {
            Object rpcService = spanAttributes.get(AttributeKey.stringKey(RPC_SERVICE));
            if (rpcService != null) {
                name = String.valueOf(rpcService);
            }
        }

        if (name.isEmpty()) {
            Object host = spanAttributes.get(AttributeKey.stringKey(HTTP_HOST));
            if (host != null) {
                name = String.valueOf(host);
            }
        }

        if (name.isEmpty()) {
            Object peer = spanAttributes.get(AttributeKey.stringKey(NET_PEER_NAME));
            if (peer != null) {
                name = String.valueOf(peer);
            }
        }

        if (name.isEmpty()) {
            name = fixSegmentName(span.getName());
        }

        if (namespace.isEmpty() && (span.getKind() == SpanKind.CLIENT || span.getKind() == SpanKind.PRODUCER)) {
            namespace = "remote";
        }

        // Build segment
        Segment segment = new Segment();
        segment.setId(span.getSpanId());
        segment.setTraceId(traceId);
        segment.setName(name);
        segment.setStartTime(startTime);
        if (inProgress != null) {
            segment.setInProgress(inProgress);
        } else {
            segment.setEndTime(endTime);
        }
        segment.setParentId(span.getParentSpanContext().isValid() ? span.getParentSpanContext().getSpanId() : null);
        segment.setFault(isFault);
        segment.setError(isError);
        segment.setThrottle(isThrottle);
        segment.setCause(causeData);
        segment.setOrigin(origin != null && !origin.isEmpty() ? origin : null);
        segment.setNamespace(namespace.isEmpty() ? null : namespace);
        segment.setUser(user != null && !user.isEmpty() ? user : null);
        segment.setHttp(httpData);
        segment.setAws(awsData);
        segment.setService(serviceData);
        segment.setSql(sqlData);
        segment.setAnnotations(annotations != null && !annotations.isEmpty() ? annotations : null);
        segment.setMetadata(metadata != null && !metadata.isEmpty() ? metadata : null);
        segment.setType(segmentType);
        segment.setLinks(spanLinks != null && !spanLinks.isEmpty() ? spanLinks : null);

        return segment;
    }

    private boolean isLocalRoot(SpanData span) {
        Object awsSpanKindVal = span.getAttributes().get(AttributeKey.stringKey(AWS_SPAN_KIND));
        return LOCAL_ROOT.equals(awsSpanKindVal);
    }

    private boolean isLocalRootSpanADependencySpan(SpanData span) {
        return span.getKind() != SpanKind.SERVER && span.getKind() != SpanKind.INTERNAL;
    }

    private void addNamespaceToSubsegmentWithRemoteService(SpanData span, Segment segment) {
        if ((span.getKind() == SpanKind.CLIENT || span.getKind() == SpanKind.CONSUMER || span.getKind() == SpanKind.PRODUCER)
                && segment.getType() != null
                && segment.getNamespace() == null) {
            Object remoteService = span.getAttributes().get(AttributeKey.stringKey(AWS_REMOTE_SERVICE));
            if (remoteService != null) {
                segment.setNamespace("remote");
            }
        }
    }

    String convertToAmazonTraceId(String traceId) throws TranslationException {
        if (traceId == null || traceId.length() != 32) {
            throw new TranslationException("Invalid trace ID: " + traceId);
        }

        long epoch = Long.parseUnsignedLong(traceId.substring(0, 8), 16);

        if (!skipTimestampValidation) {
            long epochNow = Instant.now().getEpochSecond();
            long maxAge = 60L * 60 * 24 * 28;
            long maxSkew = 60L * 5;
            long delta = epochNow - epoch;
            if (delta > maxAge || delta < -maxSkew) {
                throw new TranslationException("Invalid xray traceid: " + traceId);
            }
        }

        String epochHex = traceId.substring(0, 8);
        String identifier = traceId.substring(8);

        return "1-" + epochHex + "-" + identifier;
    }

    static String determineAwsOrigin(Attributes resourceAttributes) {
        Object provider = resourceAttributes.get(AttributeKey.stringKey(CLOUD_PROVIDER));
        if (provider != null && !CLOUD_PROVIDER_AWS.equals(String.valueOf(provider))) {
            return "";
        }

        Object platform = resourceAttributes.get(AttributeKey.stringKey(CLOUD_PLATFORM));
        if (platform != null) {
            String platformStr = String.valueOf(platform);
            switch (platformStr) {
                case CLOUD_PLATFORM_AWS_APP_RUNNER: return ORIGIN_APP_RUNNER;
                case CLOUD_PLATFORM_AWS_EKS: return ORIGIN_EKS;
                case CLOUD_PLATFORM_AWS_ELASTIC_BEANSTALK: return ORIGIN_EB;
                case CLOUD_PLATFORM_AWS_ECS:
                    Object lt = resourceAttributes.get(AttributeKey.stringKey(AWS_ECS_LAUNCHTYPE));
                    if (lt == null) return ORIGIN_ECS;
                    String launchType = String.valueOf(lt);
                    if (AWS_ECS_LAUNCHTYPE_EC2.equals(launchType)) return ORIGIN_ECS_EC2;
                    if (AWS_ECS_LAUNCHTYPE_FARGATE.equals(launchType)) return ORIGIN_ECS_FARGATE;
                    return ORIGIN_ECS;
                case CLOUD_PLATFORM_AWS_EC2: return ORIGIN_EC2;
                default: return "";
            }
        }

        return "";
    }

    private ServiceData makeService(Attributes resourceAttributes) {
        Object version = resourceAttributes.get(AttributeKey.stringKey(SERVICE_VERSION));
        if (version == null) {
            Object tags = resourceAttributes.get(AttributeKey.stringArrayKey(CONTAINER_IMAGE_TAGS));
            if (tags instanceof List && !((List<?>) tags).isEmpty()) {
                version = ((List<?>) tags).get(0);
            } else {
                version = resourceAttributes.get(AttributeKey.stringKey(CONTAINER_IMAGE_TAG));
            }
        }
        if (version != null) {
            ServiceData serviceData = new ServiceData();
            serviceData.setVersion(String.valueOf(version));
            return serviceData;
        }
        return null;
    }

    private Map<String, Object> addSpecialAttributes(Map<String, Object> attributes, Attributes spanAttributes) {
        for (String name : indexedAttrs) {
            if (!attributes.containsKey(name)) {
                Object value = spanAttributes.get(AttributeKey.stringKey(name));
                if (value != null) {
                    attributes.put(name, value);
                }
            }
        }
        return attributes;
    }

    private class XRayAttributesResult {
        final String user;
        final Map<String, Object> annotations;
        final Map<String, Map<String, Object>> metadata;

        XRayAttributesResult(String user, Map<String, Object> annotations, Map<String, Map<String, Object>> metadata) {
            this.user = user;
            this.annotations = annotations;
            this.metadata = metadata;
        }
    }

    private XRayAttributesResult makeXRayAttributes(Map<String, Object> attributes, Attributes resourceAttributes, boolean storeResource) {
        Map<String, Object> annotations = new HashMap<>();
        Map<String, Map<String, Object>> metadata = new HashMap<>();
        String user = "";

        Object userId = attributes.get(ENDUSER_ID);
        if (userId != null) {
            user = String.valueOf(userId);
            attributes.remove(ENDUSER_ID);
        }

        if (attributes.isEmpty() && (!storeResource || resourceAttributes.isEmpty())) {
            return new XRayAttributesResult(user, null, null);
        }

        Map<String, Object> defaultMetadata = new HashMap<>();

        Set<String> indexedKeys = new HashSet<>();
        if (!indexAllAttrs) {
            indexedKeys.addAll(indexedAttrs);
        }

        // Handle aws.xray.annotations attribute
        Object annotationKeysVal = attributes.get(AWS_XRAY_ANNOTATIONS);
        if (annotationKeysVal instanceof List) {
            for (Object item : (List<?>) annotationKeysVal) {
                if (item instanceof String) {
                    indexedKeys.add((String) item);
                }
            }
            attributes.remove(AWS_XRAY_ANNOTATIONS);
        }

        if (storeResource) {
            for (Map.Entry<AttributeKey<?>, Object> entry : resourceAttributes.asMap().entrySet()) {
                String key = "otel.resource." + entry.getKey().getKey();
                Object value = entry.getValue();
                boolean indexed = indexAllAttrs || indexedKeys.contains(key);
                if (indexed && isAnnotatableValue(value)) {
                    annotations.put(fixAnnotationKey(key, allowDotInAnnotationKeys), value);
                } else if (value != null) {
                    defaultMetadata.put(key, value);
                }
            }
        }

        if (indexAllAttrs) {
            for (Map.Entry<String, Object> entry : attributes.entrySet()) {
                Object value = entry.getValue();
                if (isAnnotatableValue(value)) {
                    annotations.put(fixAnnotationKey(entry.getKey(), allowDotInAnnotationKeys), value);
                }
            }
        } else {
            for (Map.Entry<String, Object> entry : attributes.entrySet()) {
                String key = entry.getKey();
                Object value = entry.getValue();
                if (indexedKeys.contains(key)) {
                    if (isAnnotatableValue(value)) {
                        annotations.put(fixAnnotationKey(key, allowDotInAnnotationKeys), value);
                    }
                } else if (key.startsWith(AWS_XRAY_METADATA_PREFIX) && value instanceof String) {
                    String ns = key.substring(AWS_XRAY_METADATA_PREFIX.length());
                    try {
                        @SuppressWarnings("unchecked")
                        Map<String, Object> metaVal = OBJECT_MAPPER.readValue((String) value, Map.class);
                        if (DEFAULT_METADATA_NAMESPACE.equalsIgnoreCase(ns)) {
                            defaultMetadata.putAll(metaVal);
                        } else {
                            metadata.put(ns, metaVal);
                        }
                    } catch (Exception e) {
                        defaultMetadata.put(key, value);
                    }
                } else if (value != null) {
                    defaultMetadata.put(key, value);
                }
            }
        }

        if (!defaultMetadata.isEmpty()) {
            metadata.put(DEFAULT_METADATA_NAMESPACE, defaultMetadata);
        }

        return new XRayAttributesResult(user, annotations, metadata);
    }

    private List<SpanLinkData> makeSpanLinks(List<LinkData> links) throws TranslationException {
        if (links == null || links.isEmpty()) {
            return null;
        }
        List<SpanLinkData> result = new ArrayList<>();
        for (LinkData link : links) {
            SpanLinkData spanLinkData = new SpanLinkData();
            spanLinkData.setSpanId(link.getSpanContext().getSpanId());
            spanLinkData.setTraceId(convertToAmazonTraceId(link.getSpanContext().getTraceId()));

            if (!link.getAttributes().isEmpty()) {
                Map<String, Object> attrs = new HashMap<>();
                for (Map.Entry<AttributeKey<?>, Object> entry : link.getAttributes().asMap().entrySet()) {
                    attrs.put(entry.getKey().getKey(), entry.getValue());
                }
                spanLinkData.setAttributes(attrs);
            }
            result.add(spanLinkData);
        }
        return result;
    }

    private static boolean isAwsSdkSpan(SpanData span) {
        Object rpcSystem = span.getAttributes().get(AttributeKey.stringKey(RPC_SYSTEM));
        return AWS_API_RPC_SYSTEM.equals(rpcSystem);
    }

    private static String trimAwsSdkPrefix(String name, SpanData span) {
        if (isAwsSdkSpan(span)) {
            if (name.startsWith("AWS.SDK.")) {
                return name.substring("AWS.SDK.".length());
            } else if (name.startsWith("AWS::")) {
                return name.substring("AWS::".length());
            }
        }
        return name;
    }

    static String fixSegmentName(String name) {
        if (INVALID_SPAN_CHARACTERS.matcher(name).find()) {
            name = INVALID_SPAN_CHARACTERS.matcher(name).replaceAll("");
        }
        if (name.length() > MAX_SEGMENT_NAME_LENGTH) {
            name = name.substring(0, MAX_SEGMENT_NAME_LENGTH);
        } else if (name.isEmpty()) {
            name = DEFAULT_SEGMENT_NAME;
        }
        return name;
    }

    static String fixAnnotationKey(String key) {
        return fixAnnotationKey(key, true);
    }

    static String fixAnnotationKey(String key, boolean allowDot) {
        StringBuilder sb = new StringBuilder(key.length());
        for (int i = 0; i < key.length(); i++) {
            char c = key.charAt(i);
            if ((c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')) {
                sb.append(c);
            } else if (allowDot && c == '.') {
                sb.append(c);
            } else {
                sb.append('_');
            }
        }
        return sb.toString();
    }

    private static boolean isAnnotatableValue(Object value) {
        return value instanceof String || value instanceof Number || value instanceof Boolean;
    }

    private SegmentTranslator(Builder builder) {
        this.indexedAttrs = builder.indexedAttrs;
        this.indexAllAttrs = builder.indexAllAttrs;
        this.logGroupNames = builder.logGroupNames;
        this.skipTimestampValidation = builder.skipTimestampValidation;
        this.allowDotInAnnotationKeys = builder.allowDotInAnnotationKeys;
    }

    public static Builder builder() {
        return new Builder();
    }

    public static class Builder {
        private List<String> indexedAttrs = Collections.emptyList();
        private boolean indexAllAttrs = false;
        private List<String> logGroupNames = Collections.emptyList();
        private boolean skipTimestampValidation = false;
        private boolean allowDotInAnnotationKeys = true;

        public Builder indexedAttrs(List<String> indexedAttrs) {
            this.indexedAttrs = indexedAttrs;
            return this;
        }

        public Builder indexAllAttrs(boolean indexAllAttrs) {
            this.indexAllAttrs = indexAllAttrs;
            return this;
        }

        public Builder logGroupNames(List<String> logGroupNames) {
            this.logGroupNames = logGroupNames;
            return this;
        }

        public Builder skipTimestampValidation(boolean skipTimestampValidation) {
            this.skipTimestampValidation = skipTimestampValidation;
            return this;
        }

        /**
         * When enabled (default: true), dots in annotation keys are preserved.
         * When disabled, dots are converted to underscores.
         * Equivalent to Go's "exporter.xray.allowDot" feature gate.
         */
        public Builder allowDotInAnnotationKeys(boolean allowDotInAnnotationKeys) {
            this.allowDotInAnnotationKeys = allowDotInAnnotationKeys;
            return this;
        }

        public SegmentTranslator build() {
            return new SegmentTranslator(this);
        }
    }
}
