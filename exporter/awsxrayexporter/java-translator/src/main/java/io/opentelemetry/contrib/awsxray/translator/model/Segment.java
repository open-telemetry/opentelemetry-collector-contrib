// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package io.opentelemetry.contrib.awsxray.translator.model;

import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.annotation.JsonProperty;
import java.util.List;
import java.util.Map;

@JsonInclude(JsonInclude.Include.NON_NULL)
public class Segment {

    @JsonProperty("name")
    private String name;

    @JsonProperty("id")
    private String id;

    @JsonProperty("start_time")
    private Double startTime;

    @JsonProperty("trace_id")
    private String traceId;

    @JsonProperty("end_time")
    private Double endTime;

    @JsonProperty("in_progress")
    private Boolean inProgress;

    @JsonProperty("fault")
    private Boolean fault;

    @JsonProperty("error")
    private Boolean error;

    @JsonProperty("throttle")
    private Boolean throttle;

    @JsonProperty("http")
    private HttpData http;

    @JsonProperty("cause")
    private CauseData cause;

    @JsonProperty("aws")
    private AwsData aws;

    @JsonProperty("service")
    private ServiceData service;

    @JsonProperty("sql")
    private SqlData sql;

    @JsonProperty("annotations")
    private Map<String, Object> annotations;

    @JsonProperty("metadata")
    private Map<String, Map<String, Object>> metadata;

    @JsonProperty("subsegments")
    private List<Segment> subsegments;

    @JsonProperty("namespace")
    private String namespace;

    @JsonProperty("parent_id")
    private String parentId;

    @JsonProperty("type")
    private String type;

    @JsonProperty("precursor_ids")
    private List<String> precursorIds;

    @JsonProperty("traced")
    private Boolean traced;

    @JsonProperty("origin")
    private String origin;

    @JsonProperty("user")
    private String user;

    @JsonProperty("resource_arn")
    private String resourceArn;

    @JsonProperty("links")
    private List<SpanLinkData> links;

    public String getName() { return name; }
    public void setName(String name) { this.name = name; }

    public String getId() { return id; }
    public void setId(String id) { this.id = id; }

    public Double getStartTime() { return startTime; }
    public void setStartTime(Double startTime) { this.startTime = startTime; }

    public String getTraceId() { return traceId; }
    public void setTraceId(String traceId) { this.traceId = traceId; }

    public Double getEndTime() { return endTime; }
    public void setEndTime(Double endTime) { this.endTime = endTime; }

    public Boolean getInProgress() { return inProgress; }
    public void setInProgress(Boolean inProgress) { this.inProgress = inProgress; }

    public Boolean getFault() { return fault; }
    public void setFault(Boolean fault) { this.fault = fault; }

    public Boolean getError() { return error; }
    public void setError(Boolean error) { this.error = error; }

    public Boolean getThrottle() { return throttle; }
    public void setThrottle(Boolean throttle) { this.throttle = throttle; }

    public HttpData getHttp() { return http; }
    public void setHttp(HttpData http) { this.http = http; }

    public CauseData getCause() { return cause; }
    public void setCause(CauseData cause) { this.cause = cause; }

    public AwsData getAws() { return aws; }
    public void setAws(AwsData aws) { this.aws = aws; }

    public ServiceData getService() { return service; }
    public void setService(ServiceData service) { this.service = service; }

    public SqlData getSql() { return sql; }
    public void setSql(SqlData sql) { this.sql = sql; }

    public Map<String, Object> getAnnotations() { return annotations; }
    public void setAnnotations(Map<String, Object> annotations) { this.annotations = annotations; }

    public Map<String, Map<String, Object>> getMetadata() { return metadata; }
    public void setMetadata(Map<String, Map<String, Object>> metadata) { this.metadata = metadata; }

    public List<Segment> getSubsegments() { return subsegments; }
    public void setSubsegments(List<Segment> subsegments) { this.subsegments = subsegments; }

    public String getNamespace() { return namespace; }
    public void setNamespace(String namespace) { this.namespace = namespace; }

    public String getParentId() { return parentId; }
    public void setParentId(String parentId) { this.parentId = parentId; }

    public String getType() { return type; }
    public void setType(String type) { this.type = type; }

    public List<String> getPrecursorIds() { return precursorIds; }
    public void setPrecursorIds(List<String> precursorIds) { this.precursorIds = precursorIds; }

    public Boolean getTraced() { return traced; }
    public void setTraced(Boolean traced) { this.traced = traced; }

    public String getOrigin() { return origin; }
    public void setOrigin(String origin) { this.origin = origin; }

    public String getUser() { return user; }
    public void setUser(String user) { this.user = user; }

    public String getResourceArn() { return resourceArn; }
    public void setResourceArn(String resourceArn) { this.resourceArn = resourceArn; }

    public List<SpanLinkData> getLinks() { return links; }
    public void setLinks(List<SpanLinkData> links) { this.links = links; }
}
