#!/usr/bin/env python3
import json
import time
import requests
import uuid

OTLP_HTTP_LOGS_ENDPOINT = "http://localhost:4318/v1/logs"  # from receivers.otlp.protocols.http
OTLP_HTTP_METRICS_ENDPOINT = "http://localhost:4318/v1/metrics"
OTLP_HTTP_TRACES_ENDPOINT = "http://localhost:4318/v1/traces"

def make_otlp_logs_json(host_name: str, host_region: str, message: str) -> dict:
    now_unix_nanos = int(time.time() * 1e9)
    return {
        "resourceLogs": [
            {
                "resource": {
                    "attributes": [
                        {"key": "host.name", "value": {"stringValue": host_name}},
                        {"key": "host.region", "value": {"stringValue": host_region}},
                    ]
                },
                "scopeLogs": [
                    {
                        "logRecords": [
                            {
                                "timeUnixNano": str(now_unix_nanos),
                                "body": {"stringValue": message},
                                "severityText": "INFO",
                                "severityNumber": 9
                            }
                        ]
                    }
                ]
            }
        ]
    }

def make_otlp_metrics_json(host_name: str, host_region: str, value: float) -> dict:
    now_unix_nanos = int(time.time() * 1e9)
    return {
        "resourceMetrics": [
            {
                "resource": {
                    "attributes": [
                        {"key": "host.name", "value": {"stringValue": host_name}},
                        {"key": "host.region", "value": {"stringValue": host_region}},
                    ]
                },
                "scopeMetrics": [
                    {
                        "metrics": [
                            {
                                "name": "my_gauge_metric",
                                "description": "A test gauge metric",
                                "unit": "1",
                                "gauge": {
                                    "dataPoints": [
                                        {
                                            "timeUnixNano": str(now_unix_nanos),
                                            "value": value,
                                            "attributes": [
                                                {"key": "metric.attribute", "value": {"stringValue": "example"}}
                                            ]
                                        }
                                    ]
                                }
                            }
                        ]
                    }
                ]
            }
        ]
    }

def make_otlp_traces_json(host_name: str, host_region: str, span_name: str) -> dict:
    now_unix_nanos = int(time.time() * 1e9)
    start_time_unix_nanos = now_unix_nanos - 1000000000 # 1 second ago
    end_time_unix_nanos = now_unix_nanos

    trace_id = uuid.uuid4().bytes.hex()
    span_id = uuid.uuid4().bytes[:8].hex()

    return {
        "resourceSpans": [
            {
                "resource": {
                    "attributes": [
                        {"key": "host.name", "value": {"stringValue": host_name}},
                        {"key": "host.region", "value": {"stringValue": host_region}},
                    ]
                },
                "scopeSpans": [
                    {
                        "spans": [
                            {
                                "traceId": trace_id,
                                "spanId": span_id,
                                "name": span_name,
                                "kind": "SPAN_KIND_SERVER",
                                "startTimeUnixNano": str(start_time_unix_nanos),
                                "endTimeUnixNano": str(end_time_unix_nanos),
                                "attributes": [
                                    {"key": "http.method", "value": {"stringValue": "GET"}},
                                    {"key": "http.status_code", "value": {"intValue": 200}}
                                ]
                            }
                        ]
                    }
                ]
            }
        ]
    }

def send_otlp_data(endpoint: str, payload: dict, data_type: str) -> None:
    headers = {"Content-Type": "application/json"}
    resp = requests.post(endpoint, headers=headers, data=json.dumps(payload))
    resp.raise_for_status()
    print(f"Sent {data_type}. Status={resp.status_code}")

def send_log(host_name: str, host_region: str, message: str) -> None:
    payload = make_otlp_logs_json(host_name, host_region, message)
    send_otlp_data(OTLP_HTTP_LOGS_ENDPOINT, payload, "log")

def send_metric(host_name: str, host_region: str, value: float) -> None:
    payload = make_otlp_metrics_json(host_name, host_region, value)
    send_otlp_data(OTLP_HTTP_METRICS_ENDPOINT, payload, "metric")

def send_trace(host_name: str, host_region: str, span_name: str) -> None:
    payload = make_otlp_traces_json(host_name, host_region, span_name)
    send_otlp_data(OTLP_HTTP_TRACES_ENDPOINT, payload, "trace")

if __name__ == "__main__":
    # Adjust values as needed; these drive your MQTT topic template substitutions.
    common_host_name = "test-host-01"
    common_host_region = "eu-west-1"

    print("Sending log...")
    send_log(
        host_name=common_host_name,
        host_region=common_host_region,
        message="hello from OTLP/HTTP JSON"
    )
    time.sleep(1) # Give collector a moment

    print("\nSending metric...")
    send_metric(
        host_name=common_host_name,
        host_region=common_host_region,
        value=42.5
    )
    time.sleep(1) # Give collector a moment

    print("\nSending trace...")
    send_trace(
        host_name=common_host_name,
        host_region=common_host_region,
        span_name="my-service-operation"
    )
