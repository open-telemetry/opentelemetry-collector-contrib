# Copyright 2018, OpenCensus Authors
# Copyright The OpenTelemetry Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

"""
The **OpenTelemetry Jaeger Exporter** allows to export `OpenTelemetry`_ traces to `Jaeger`_.
This exporter always send traces to the configured agent using Thrift compact protocol over UDP.
An optional collector can be configured, in this case Thrift binary protocol over HTTP is used.
gRPC is still not supported by this implementation.

Usage
-----

.. code:: python

    from opentelemetry import trace
    from opentelemetry.exporter import jaeger
    from opentelemetry.sdk.trace import TracerProvider
    from opentelemetry.sdk.trace.export import BatchExportSpanProcessor

    trace.set_tracer_provider(TracerProvider())
    tracer = trace.get_tracer(__name__)

    # create a JaegerSpanExporter
    jaeger_exporter = jaeger.JaegerSpanExporter(
        service_name='my-helloworld-service',
        # configure agent
        agent_host_name='localhost',
        agent_port=6831,
        # optional: configure also collector
        # collector_host_name='localhost',
        # collector_port=14268,
        # collector_endpoint='/api/traces?format=jaeger.thrift',
        # collector_protocol='http',
        # username=xxxx, # optional
        # password=xxxx, # optional
    )

    # Create a BatchExportSpanProcessor and add the exporter to it
    span_processor = BatchExportSpanProcessor(jaeger_exporter)

    # add to the tracer
    trace.get_tracer_provider().add_span_processor(span_processor)

    with tracer.start_as_current_span('foo'):
        print('Hello world!')

API
---
.. _Jaeger: https://www.jaegertracing.io/
.. _OpenTelemetry: https://github.com/open-telemetry/opentelemetry-python/
"""

import base64
import logging
import socket

from thrift.protocol import TBinaryProtocol, TCompactProtocol
from thrift.transport import THttpClient, TTransport

import opentelemetry.trace as trace_api
from opentelemetry.exporter.jaeger.gen.agent import Agent as agent
from opentelemetry.exporter.jaeger.gen.jaeger import Collector as jaeger
from opentelemetry.sdk.trace.export import Span, SpanExporter, SpanExportResult
from opentelemetry.trace.status import StatusCanonicalCode

DEFAULT_AGENT_HOST_NAME = "localhost"
DEFAULT_AGENT_PORT = 6831
DEFAULT_COLLECTOR_ENDPOINT = "/api/traces?format=jaeger.thrift"
DEFAULT_COLLECTOR_PROTOCOL = "http"

UDP_PACKET_MAX_LENGTH = 65000

logger = logging.getLogger(__name__)


class JaegerSpanExporter(SpanExporter):
    """Jaeger span exporter for OpenTelemetry.

    Args:
        service_name: Service that logged an annotation in a trace.Classifier
            when query for spans.
        agent_host_name: The host name of the Jaeger-Agent.
        agent_port: The port of the Jaeger-Agent.
        collector_host_name: The host name of the Jaeger-Collector HTTP/HTTPS
            Thrift.
        collector_port: The port of the Jaeger-Collector HTTP/HTTPS Thrift.
        collector_endpoint: The endpoint of the Jaeger-Collector HTTP/HTTPS Thrift.
        collector_protocol: The transfer protocol for the Jaeger-Collector(HTTP or HTTPS).
        username: The user name of the Basic Auth if authentication is
            required.
        password: The password of the Basic Auth if authentication is
            required.
    """

    def __init__(
        self,
        service_name,
        agent_host_name=DEFAULT_AGENT_HOST_NAME,
        agent_port=DEFAULT_AGENT_PORT,
        collector_host_name=None,
        collector_port=None,
        collector_endpoint=DEFAULT_COLLECTOR_ENDPOINT,
        collector_protocol=DEFAULT_COLLECTOR_PROTOCOL,
        username=None,
        password=None,
    ):
        self.service_name = service_name
        self.agent_host_name = agent_host_name
        self.agent_port = agent_port
        self._agent_client = None
        self.collector_host_name = collector_host_name
        self.collector_port = collector_port
        self.collector_endpoint = collector_endpoint
        self.collector_protocol = collector_protocol
        self.username = username
        self.password = password
        self._collector = None

    @property
    def agent_client(self):
        if self._agent_client is None:
            self._agent_client = AgentClientUDP(
                host_name=self.agent_host_name, port=self.agent_port
            )
        return self._agent_client

    @property
    def collector(self):
        if self._collector is not None:
            return self._collector

        if self.collector_host_name is None or self.collector_port is None:
            return None

        thrift_url = "{}://{}:{}{}".format(
            self.collector_protocol,
            self.collector_host_name,
            self.collector_port,
            self.collector_endpoint,
        )

        auth = None
        if self.username is not None and self.password is not None:
            auth = (self.username, self.password)

        self._collector = Collector(thrift_url=thrift_url, auth=auth)
        return self._collector

    def export(self, spans):
        jaeger_spans = _translate_to_jaeger(spans)

        batch = jaeger.Batch(
            spans=jaeger_spans,
            process=jaeger.Process(serviceName=self.service_name),
        )

        if self.collector is not None:
            self.collector.submit(batch)
        else:
            self.agent_client.emit(batch)

        return SpanExportResult.SUCCESS

    def shutdown(self):
        pass


def _nsec_to_usec_round(nsec):
    """Round nanoseconds to microseconds"""
    return (nsec + 500) // 10 ** 3


def _translate_to_jaeger(spans: Span):
    """Translate the spans to Jaeger format.

    Args:
        spans: Tuple of spans to convert
    """

    jaeger_spans = []

    for span in spans:
        ctx = span.get_span_context()
        trace_id = ctx.trace_id
        span_id = ctx.span_id

        start_time_us = _nsec_to_usec_round(span.start_time)
        duration_us = _nsec_to_usec_round(span.end_time - span.start_time)

        status = span.status

        parent_id = span.parent.span_id if span.parent else 0

        tags = _extract_tags(span.attributes)
        tags.extend(_extract_tags(span.resource.attributes))

        tags.extend(
            [
                _get_long_tag("status.code", status.canonical_code.value),
                _get_string_tag("status.message", status.description),
                _get_string_tag("span.kind", span.kind.name),
            ]
        )

        if span.instrumentation_info is not None:
            tags.extend(
                [
                    _get_string_tag(
                        "otel.instrumentation_library.name",
                        span.instrumentation_info.name,
                    ),
                    _get_string_tag(
                        "otel.instrumentation_library.version",
                        span.instrumentation_info.version,
                    ),
                ]
            )

        # Ensure that if Status.Code is not OK, that we set the "error" tag on the Jaeger span.
        if status.canonical_code is not StatusCanonicalCode.OK:
            tags.append(_get_bool_tag("error", True))

        refs = _extract_refs_from_span(span)
        logs = _extract_logs_from_span(span)

        flags = int(ctx.trace_flags)

        jaeger_span = jaeger.Span(
            traceIdHigh=_get_trace_id_high(trace_id),
            traceIdLow=_get_trace_id_low(trace_id),
            # generated code expects i64
            spanId=_convert_int_to_i64(span_id),
            operationName=span.name,
            startTime=start_time_us,
            duration=duration_us,
            tags=tags,
            logs=logs,
            references=refs,
            flags=flags,
            parentSpanId=_convert_int_to_i64(parent_id),
        )

        jaeger_spans.append(jaeger_span)

    return jaeger_spans


def _extract_refs_from_span(span):
    if not span.links:
        return None

    refs = []
    for link in span.links:
        trace_id = link.context.trace_id
        span_id = link.context.span_id
        refs.append(
            jaeger.SpanRef(
                refType=jaeger.SpanRefType.FOLLOWS_FROM,
                traceIdHigh=_get_trace_id_high(trace_id),
                traceIdLow=_get_trace_id_low(trace_id),
                spanId=_convert_int_to_i64(span_id),
            )
        )
    return refs


def _convert_int_to_i64(val):
    """Convert integer to signed int64 (i64)"""
    if val > 0x7FFFFFFFFFFFFFFF:
        val -= 0x10000000000000000
    return val


def _get_trace_id_low(trace_id):
    return _convert_int_to_i64(trace_id & 0xFFFFFFFFFFFFFFFF)


def _get_trace_id_high(trace_id):
    return _convert_int_to_i64((trace_id >> 64) & 0xFFFFFFFFFFFFFFFF)


def _extract_logs_from_span(span):
    if not span.events:
        return None

    logs = []

    for event in span.events:
        fields = _extract_tags(event.attributes)

        fields.append(
            jaeger.Tag(
                key="message", vType=jaeger.TagType.STRING, vStr=event.name
            )
        )

        event_timestamp_us = _nsec_to_usec_round(event.timestamp)
        logs.append(
            jaeger.Log(timestamp=int(event_timestamp_us), fields=fields)
        )
    return logs


def _extract_tags(attr):
    if not attr:
        return []
    tags = []
    for attribute_key, attribute_value in attr.items():
        tag = _convert_attribute_to_tag(attribute_key, attribute_value)
        if tag is None:
            continue
        tags.append(tag)
    return tags


def _convert_attribute_to_tag(key, attr):
    """Convert the attributes to jaeger tags."""
    if isinstance(attr, bool):
        return jaeger.Tag(key=key, vBool=attr, vType=jaeger.TagType.BOOL)
    if isinstance(attr, str):
        return jaeger.Tag(key=key, vStr=attr, vType=jaeger.TagType.STRING)
    if isinstance(attr, int):
        return jaeger.Tag(key=key, vLong=attr, vType=jaeger.TagType.LONG)
    if isinstance(attr, float):
        return jaeger.Tag(key=key, vDouble=attr, vType=jaeger.TagType.DOUBLE)
    if isinstance(attr, tuple):
        return jaeger.Tag(key=key, vStr=str(attr), vType=jaeger.TagType.STRING)
    logger.warning("Could not serialize attribute %s:%r to tag", key, attr)
    return None


def _get_long_tag(key, val):
    return jaeger.Tag(key=key, vLong=val, vType=jaeger.TagType.LONG)


def _get_string_tag(key, val):
    return jaeger.Tag(key=key, vStr=val, vType=jaeger.TagType.STRING)


def _get_bool_tag(key, val):
    return jaeger.Tag(key=key, vBool=val, vType=jaeger.TagType.BOOL)


class AgentClientUDP:
    """Implement a UDP client to agent.

    Args:
        host_name: The host name of the Jaeger server.
        port: The port of the Jaeger server.
        max_packet_size: Maximum size of UDP packet.
        client: Class for creating new client objects for agencies.
    """

    def __init__(
        self,
        host_name,
        port,
        max_packet_size=UDP_PACKET_MAX_LENGTH,
        client=agent.Client,
    ):
        self.address = (host_name, port)
        self.max_packet_size = max_packet_size
        self.buffer = TTransport.TMemoryBuffer()
        self.client = client(
            iprot=TCompactProtocol.TCompactProtocol(trans=self.buffer)
        )

    def emit(self, batch: jaeger.Batch):
        """
        Args:
            batch: Object to emit Jaeger spans.
        """

        # pylint: disable=protected-access
        self.client._seqid = 0
        #  truncate and reset the position of BytesIO object
        self.buffer._buffer.truncate(0)
        self.buffer._buffer.seek(0)
        self.client.emitBatch(batch)
        buff = self.buffer.getvalue()
        if len(buff) > self.max_packet_size:
            logger.warning(
                "Data exceeds the max UDP packet size; size %r, max %r",
                len(buff),
                self.max_packet_size,
            )
            return

        with socket.socket(socket.AF_INET, socket.SOCK_DGRAM) as udp_socket:
            udp_socket.sendto(buff, self.address)


class Collector:
    """Submits collected spans to Thrift HTTP server.

    Args:
        thrift_url: URL of the Jaeger HTTP Thrift.
        auth: Auth tuple that contains username and password for Basic Auth.
    """

    def __init__(self, thrift_url="", auth=None):
        self.thrift_url = thrift_url
        self.auth = auth
        self.http_transport = THttpClient.THttpClient(
            uri_or_host=self.thrift_url
        )
        self.protocol = TBinaryProtocol.TBinaryProtocol(self.http_transport)

        # set basic auth header
        if auth is not None:
            auth_header = "{}:{}".format(*auth)
            decoded = base64.b64encode(auth_header.encode()).decode("ascii")
            basic_auth = dict(Authorization="Basic {}".format(decoded))
            self.http_transport.setCustomHeaders(basic_auth)

    def submit(self, batch: jaeger.Batch):
        """Submits batches to Thrift HTTP Server through Binary Protocol.

        Args:
            batch: Object to emit Jaeger spans.
        """
        batch.write(self.protocol)
        self.http_transport.flush()
        code = self.http_transport.code
        msg = self.http_transport.message
        if code >= 300 or code < 200:
            logger.error(
                "Traces cannot be uploaded; HTTP status code: %s, message: %s",
                code,
                msg,
            )
