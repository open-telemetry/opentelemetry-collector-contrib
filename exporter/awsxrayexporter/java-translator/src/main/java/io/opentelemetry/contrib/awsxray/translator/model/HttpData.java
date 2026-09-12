// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package io.opentelemetry.contrib.awsxray.translator.model;

import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.annotation.JsonProperty;

@JsonInclude(JsonInclude.Include.NON_NULL)
public class HttpData {

    @JsonProperty("request")
    private RequestData request;

    @JsonProperty("response")
    private ResponseData response;

    public RequestData getRequest() { return request; }
    public void setRequest(RequestData request) { this.request = request; }

    public ResponseData getResponse() { return response; }
    public void setResponse(ResponseData response) { this.response = response; }

    @JsonInclude(JsonInclude.Include.NON_NULL)
    public static class RequestData {
        @JsonProperty("method")
        private String method;

        @JsonProperty("url")
        private String url;

        @JsonProperty("user_agent")
        private String userAgent;

        @JsonProperty("client_ip")
        private String clientIp;

        @JsonProperty("x_forwarded_for")
        private Boolean xForwardedFor;

        public String getMethod() { return method; }
        public void setMethod(String method) { this.method = method; }

        public String getUrl() { return url; }
        public void setUrl(String url) { this.url = url; }

        public String getUserAgent() { return userAgent; }
        public void setUserAgent(String userAgent) { this.userAgent = userAgent; }

        public String getClientIp() { return clientIp; }
        public void setClientIp(String clientIp) { this.clientIp = clientIp; }

        public Boolean getXForwardedFor() { return xForwardedFor; }
        public void setXForwardedFor(Boolean xForwardedFor) { this.xForwardedFor = xForwardedFor; }
    }

    @JsonInclude(JsonInclude.Include.NON_NULL)
    public static class ResponseData {
        @JsonProperty("status")
        private Long status;

        @JsonProperty("content_length")
        private Long contentLength;

        public Long getStatus() { return status; }
        public void setStatus(Long status) { this.status = status; }

        public Long getContentLength() { return contentLength; }
        public void setContentLength(Long contentLength) { this.contentLength = contentLength; }
    }
}
