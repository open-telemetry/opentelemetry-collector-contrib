// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package io.opentelemetry.contrib.awsxray.translator;

import io.opentelemetry.api.common.AttributeKey;
import io.opentelemetry.api.common.Attributes;
import io.opentelemetry.api.trace.SpanKind;
import io.opentelemetry.contrib.awsxray.translator.model.HttpData;
import io.opentelemetry.sdk.trace.data.EventData;
import io.opentelemetry.sdk.trace.data.SpanData;
import java.net.InetAddress;
import java.util.HashMap;
import java.util.List;
import java.util.Map;

import static io.opentelemetry.contrib.awsxray.translator.AttributeConstants.*;

final class HttpTranslator {

    static class HttpResult {
        final Map<String, Object> filteredAttributes;
        final HttpData httpData;

        HttpResult(Map<String, Object> filteredAttributes, HttpData httpData) {
            this.filteredAttributes = filteredAttributes;
            this.httpData = httpData;
        }
    }

    static HttpResult makeHttp(SpanData span) {
        Map<String, Object> filtered = new HashMap<>();
        Map<String, String> urlParts = new HashMap<>();

        HttpData.RequestData request = new HttpData.RequestData();
        HttpData.ResponseData response = new HttpData.ResponseData();

        Attributes attributes = span.getAttributes();
        if (attributes.isEmpty()) {
            return new HttpResult(filtered, null);
        }

        boolean hasHttp = false;
        boolean hasHttpRequestUrlAttributes = false;
        boolean hasNetPeerAddr = false;

        for (Map.Entry<AttributeKey<?>, Object> entry : attributes.asMap().entrySet()) {
            String key = entry.getKey().getKey();
            Object value = entry.getValue();

            switch (key) {
                case HTTP_METHOD:
                case HTTP_REQUEST_METHOD:
                    request.setMethod(String.valueOf(value));
                    hasHttp = true;
                    break;
                case HTTP_CLIENT_IP:
                    request.setClientIp(String.valueOf(value));
                    hasHttp = true;
                    break;
                case HTTP_USER_AGENT:
                case USER_AGENT_ORIGINAL:
                    request.setUserAgent(String.valueOf(value));
                    hasHttp = true;
                    break;
                case HTTP_STATUS_CODE:
                case HTTP_RESPONSE_STATUS_CODE:
                    response.setStatus(((Number) value).longValue());
                    hasHttp = true;
                    break;
                case HTTP_URL:
                case URL_FULL:
                    urlParts.put(HTTP_URL, String.valueOf(value));
                    hasHttp = true;
                    hasHttpRequestUrlAttributes = true;
                    break;
                case HTTP_SCHEME:
                case URL_SCHEME:
                    urlParts.put(HTTP_SCHEME, String.valueOf(value));
                    hasHttp = true;
                    break;
                case HTTP_HOST:
                    urlParts.put(key, String.valueOf(value));
                    hasHttp = true;
                    hasHttpRequestUrlAttributes = true;
                    break;
                case HTTP_TARGET:
                    urlParts.put(key, String.valueOf(value));
                    hasHttp = true;
                    break;
                case HTTP_SERVER_NAME:
                    urlParts.put(key, String.valueOf(value));
                    hasHttp = true;
                    hasHttpRequestUrlAttributes = true;
                    break;
                case NET_HOST_PORT:
                    urlParts.put(key, String.valueOf(value));
                    hasHttp = true;
                    break;
                case HOST_NAME:
                case NET_HOST_NAME:
                case NET_PEER_NAME:
                    urlParts.put(key, String.valueOf(value));
                    hasHttpRequestUrlAttributes = true;
                    break;
                case NET_PEER_PORT:
                    urlParts.put(key, String.valueOf(value));
                    break;
                case NET_PEER_IP:
                    if (request.getClientIp() == null) {
                        request.setClientIp(String.valueOf(value));
                    }
                    urlParts.put(key, String.valueOf(value));
                    hasHttpRequestUrlAttributes = true;
                    hasNetPeerAddr = true;
                    break;
                case NETWORK_PEER_ADDRESS:
                    if (isValidIpAddress(String.valueOf(value))) {
                        if (request.getClientIp() == null) {
                            request.setClientIp(String.valueOf(value));
                        }
                        hasHttpRequestUrlAttributes = true;
                        hasNetPeerAddr = true;
                    }
                    break;
                case CLIENT_ADDRESS:
                    if (isValidIpAddress(String.valueOf(value))) {
                        request.setClientIp(String.valueOf(value));
                    }
                    break;
                case URL_PATH:
                    urlParts.put(key, String.valueOf(value));
                    hasHttp = true;
                    break;
                case URL_QUERY:
                    urlParts.put(key, String.valueOf(value));
                    hasHttp = true;
                    break;
                case SERVER_ADDRESS:
                    urlParts.put(key, String.valueOf(value));
                    hasHttpRequestUrlAttributes = true;
                    break;
                case SERVER_PORT:
                    urlParts.put(key, String.valueOf(value));
                    break;
                default:
                    filtered.put(key, value);
                    break;
            }
        }

        if (!hasNetPeerAddr && request.getClientIp() != null) {
            request.setXForwardedFor(true);
        }

        if (!hasHttp) {
            return new HttpResult(filtered, null);
        }

        if (hasHttpRequestUrlAttributes) {
            if (span.getKind() == SpanKind.SERVER) {
                request.setUrl(constructServerUrl(urlParts));
            } else {
                request.setUrl(constructClientUrl(urlParts));
            }
        }

        long responseSize = extractResponseSizeFromEvents(span);
        if (responseSize > 0) {
            response.setContentLength(responseSize);
        }

        HttpData httpData = new HttpData();
        httpData.setRequest(request);
        httpData.setResponse(response);

        return new HttpResult(filtered, httpData);
    }

    private static long extractResponseSizeFromEvents(SpanData span) {
        long size = extractResponseSizeFromAttributes(span.getAttributes());
        if (size != 0) {
            return size;
        }
        List<EventData> events = span.getEvents();
        for (EventData event : events) {
            size = extractResponseSizeFromAttributes(event.getAttributes());
            if (size != 0) {
                return size;
            }
        }
        return 0;
    }

    private static long extractResponseSizeFromAttributes(Attributes attributes) {
        Object typeVal = attributes.get(AttributeKey.stringKey("message.type"));
        if ("RECEIVED".equals(typeVal)) {
            Object sizeVal = attributes.get(AttributeKey.longKey("messaging.message.payload_size_bytes"));
            if (sizeVal instanceof Long) {
                return (Long) sizeVal;
            }
        }
        return 0;
    }

    static String constructClientUrl(Map<String, String> urlParts) {
        String url = urlParts.get(HTTP_URL);
        if (url != null) {
            return url;
        }

        String scheme = urlParts.getOrDefault(HTTP_SCHEME, "http");
        String host = urlParts.get(HTTP_HOST);
        String port = "";
        if (host == null) {
            host = urlParts.get(NET_PEER_NAME);
            if (host == null) {
                host = urlParts.getOrDefault(NET_PEER_IP, "");
            }
            port = urlParts.getOrDefault(NET_PEER_PORT, "");
        }

        StringBuilder sb = new StringBuilder();
        sb.append(scheme).append("://").append(host);
        if (!port.isEmpty() && !isDefaultPort(scheme, port)) {
            sb.append(":").append(port);
        }

        String target = urlParts.get(HTTP_TARGET);
        if (target != null) {
            sb.append(target);
        } else {
            String path = urlParts.get(URL_PATH);
            if (path != null) {
                sb.append(path);
            } else {
                sb.append("/");
            }
            String query = urlParts.get(URL_QUERY);
            if (query != null) {
                if (!query.startsWith("?")) {
                    sb.append("?");
                }
                sb.append(query);
            }
        }
        return sb.toString();
    }

    static String constructServerUrl(Map<String, String> urlParts) {
        String url = urlParts.get(HTTP_URL);
        if (url != null) {
            return url;
        }

        String scheme = urlParts.getOrDefault(HTTP_SCHEME, "http");
        String host = urlParts.get(HTTP_HOST);
        String port = "";
        if (host == null) {
            host = urlParts.get(HTTP_SERVER_NAME);
            if (host == null) {
                host = urlParts.get(NET_HOST_NAME);
                if (host == null) {
                    host = urlParts.get(HOST_NAME);
                    if (host == null) {
                        host = urlParts.getOrDefault(SERVER_ADDRESS, "");
                    }
                }
            }
            port = urlParts.get(NET_HOST_PORT);
            if (port == null) {
                port = urlParts.getOrDefault(SERVER_PORT, "");
            }
        }

        StringBuilder sb = new StringBuilder();
        sb.append(scheme).append("://").append(host);
        if (!port.isEmpty() && !isDefaultPort(scheme, port)) {
            sb.append(":").append(port);
        }

        String target = urlParts.get(HTTP_TARGET);
        if (target != null) {
            sb.append(target);
        } else {
            String path = urlParts.get(URL_PATH);
            if (path != null) {
                sb.append(path);
            } else {
                sb.append("/");
            }
            String query = urlParts.get(URL_QUERY);
            if (query != null) {
                if (!query.startsWith("?")) {
                    sb.append("?");
                }
                sb.append(query);
            }
        }
        return sb.toString();
    }

    private static boolean isDefaultPort(String scheme, String port) {
        return ("http".equals(scheme) && "80".equals(port))
                || ("https".equals(scheme) && "443".equals(port));
    }

    private static boolean isValidIpAddress(String ip) {
        try {
            InetAddress.getByName(ip);
            return !ip.isEmpty();
        } catch (Exception e) {
            return false;
        }
    }

    private HttpTranslator() {}
}
