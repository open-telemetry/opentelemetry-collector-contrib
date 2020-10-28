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

import logging
import os
from urllib.parse import urlparse

from ddtrace.ext import SpanTypes as DatadogSpanTypes
from ddtrace.internal.writer import AgentWriter
from ddtrace.span import Span as DatadogSpan

import opentelemetry.trace as trace_api
from opentelemetry.sdk.trace import sampling
from opentelemetry.sdk.trace.export import SpanExporter, SpanExportResult

# pylint:disable=relative-beyond-top-level
from .constants import (
    DD_ORIGIN,
    ENV_KEY,
    SAMPLE_RATE_METRIC_KEY,
    SERVICE_NAME_TAG,
    VERSION_KEY,
)

logger = logging.getLogger(__name__)


DEFAULT_AGENT_URL = "http://localhost:8126"
_INSTRUMENTATION_SPAN_TYPES = {
    "opentelemetry.instrumentation.aiohttp-client": DatadogSpanTypes.HTTP,
    "opentelemetry.instrumentation.asgi": DatadogSpanTypes.WEB,
    "opentelemetry.instrumentation.dbapi": DatadogSpanTypes.SQL,
    "opentelemetry.instrumentation.django": DatadogSpanTypes.WEB,
    "opentelemetry.instrumentation.flask": DatadogSpanTypes.WEB,
    "opentelemetry.instrumentation.grpc": DatadogSpanTypes.GRPC,
    "opentelemetry.instrumentation.jinja2": DatadogSpanTypes.TEMPLATE,
    "opentelemetry.instrumentation.mysql": DatadogSpanTypes.SQL,
    "opentelemetry.instrumentation.psycopg2": DatadogSpanTypes.SQL,
    "opentelemetry.instrumentation.pymemcache": DatadogSpanTypes.CACHE,
    "opentelemetry.instrumentation.pymongo": DatadogSpanTypes.MONGODB,
    "opentelemetry.instrumentation.pymysql": DatadogSpanTypes.SQL,
    "opentelemetry.instrumentation.redis": DatadogSpanTypes.REDIS,
    "opentelemetry.instrumentation.requests": DatadogSpanTypes.HTTP,
    "opentelemetry.instrumentation.sqlalchemy": DatadogSpanTypes.SQL,
    "opentelemetry.instrumentation.wsgi": DatadogSpanTypes.WEB,
}


class DatadogSpanExporter(SpanExporter):
    """Datadog span exporter for OpenTelemetry.

    Args:
        agent_url: The url of the Datadog Agent or use ``DD_TRACE_AGENT_URL`` environment variable
        service: The service name to be used for the application or use ``DD_SERVICE`` environment variable
        env: Set the application’s environment or use ``DD_ENV`` environment variable
        version: Set the application’s version or use ``DD_VERSION`` environment variable
        tags: A list of default tags to be added to every span or use ``DD_TAGS`` environment variable
    """

    def __init__(
        self, agent_url=None, service=None, env=None, version=None, tags=None
    ):
        self.agent_url = (
            agent_url
            if agent_url
            else os.environ.get("DD_TRACE_AGENT_URL", DEFAULT_AGENT_URL)
        )
        self.service = service or os.environ.get("DD_SERVICE")
        self.env = env or os.environ.get("DD_ENV")
        self.version = version or os.environ.get("DD_VERSION")
        self.tags = _parse_tags_str(tags or os.environ.get("DD_TAGS"))
        self._agent_writer = None

    @property
    def agent_writer(self):
        if self._agent_writer is None:
            url_parsed = urlparse(self.agent_url)
            if url_parsed.scheme in ("http", "https"):
                self._agent_writer = AgentWriter(
                    hostname=url_parsed.hostname,
                    port=url_parsed.port,
                    https=url_parsed.scheme == "https",
                )
            elif url_parsed.scheme == "unix":
                self._agent_writer = AgentWriter(uds_path=url_parsed.path)
            else:
                raise ValueError(
                    "Unknown scheme `%s` for agent URL" % url_parsed.scheme
                )
        return self._agent_writer

    def export(self, spans):
        datadog_spans = self._translate_to_datadog(spans)

        self.agent_writer.write(spans=datadog_spans)

        return SpanExportResult.SUCCESS

    def shutdown(self):
        if self.agent_writer.started:
            self.agent_writer.stop()
            self.agent_writer.join(self.agent_writer.exit_timeout)

    # pylint: disable=too-many-locals
    def _translate_to_datadog(self, spans):
        datadog_spans = []

        for span in spans:
            trace_id, parent_id, span_id = _get_trace_ids(span)

            # datadog Span is initialized with a reference to the tracer which is
            # used to record the span when it is finished. We can skip ignore this
            # because we are not calling the finish method and explictly set the
            # duration.
            tracer = None

            # extract resource attributes to be used as tags as well as potential service name
            [
                resource_tags,
                resource_service_name,
            ] = _extract_tags_from_resource(span.resource)

            datadog_span = DatadogSpan(
                tracer,
                _get_span_name(span),
                service=resource_service_name or self.service,
                resource=_get_resource(span),
                span_type=_get_span_type(span),
                trace_id=trace_id,
                span_id=span_id,
                parent_id=parent_id,
            )
            datadog_span.start_ns = span.start_time
            datadog_span.duration_ns = span.end_time - span.start_time

            if not span.status.is_ok:
                datadog_span.error = 1
                if span.status.description:
                    exc_type, exc_val = _get_exc_info(span)
                    # no mapping for error.stack since traceback not recorded
                    datadog_span.set_tag("error.msg", exc_val)
                    datadog_span.set_tag("error.type", exc_type)

            # combine resource attributes and span attributes, don't modify existing span attributes
            combined_span_tags = {}
            combined_span_tags.update(resource_tags)
            combined_span_tags.update(span.attributes)

            datadog_span.set_tags(combined_span_tags)

            # add configured env tag
            if self.env is not None:
                datadog_span.set_tag(ENV_KEY, self.env)

            # add configured application version tag to only root span
            if self.version is not None and parent_id == 0:
                datadog_span.set_tag(VERSION_KEY, self.version)

            # add configured global tags
            datadog_span.set_tags(self.tags)

            # add origin to root span
            origin = _get_origin(span)
            if origin and parent_id == 0:
                datadog_span.set_tag(DD_ORIGIN, origin)

            sampling_rate = _get_sampling_rate(span)
            if sampling_rate is not None:
                datadog_span.set_metric(SAMPLE_RATE_METRIC_KEY, sampling_rate)

            # span events and span links are not supported

            datadog_spans.append(datadog_span)

        return datadog_spans


def _get_trace_ids(span):
    """Extract tracer ids from span"""
    ctx = span.get_span_context()
    trace_id = ctx.trace_id
    span_id = ctx.span_id

    if isinstance(span.parent, trace_api.Span):
        parent_id = span.parent.get_span_context().span_id
    elif isinstance(span.parent, trace_api.SpanContext):
        parent_id = span.parent.span_id
    else:
        parent_id = 0

    trace_id = _convert_trace_id_uint64(trace_id)

    return trace_id, parent_id, span_id


def _convert_trace_id_uint64(otel_id):
    """Convert 128-bit int used for trace_id to 64-bit unsigned int"""
    return otel_id & 0xFFFFFFFFFFFFFFFF


def _get_span_name(span):
    """Get span name by using instrumentation and kind while backing off to
    span.name
    """
    instrumentation_name = (
        span.instrumentation_info.name if span.instrumentation_info else None
    )
    span_kind_name = span.kind.name if span.kind else None
    name = (
        "{}.{}".format(instrumentation_name, span_kind_name)
        if instrumentation_name and span_kind_name
        else span.name
    )
    return name


def _get_resource(span):
    """Get resource name for span"""
    if "http.method" in span.attributes:
        route = span.attributes.get("http.route")
        return (
            span.attributes["http.method"] + " " + route
            if route
            else span.attributes["http.method"]
        )

    return span.name


def _get_span_type(span):
    """Get Datadog span type"""
    instrumentation_name = (
        span.instrumentation_info.name if span.instrumentation_info else None
    )
    span_type = _INSTRUMENTATION_SPAN_TYPES.get(instrumentation_name)
    return span_type


def _get_exc_info(span):
    """Parse span status description for exception type and value"""
    exc_type, exc_val = span.status.description.split(":", 1)
    return exc_type, exc_val.strip()


def _get_origin(span):
    ctx = span.get_span_context()
    origin = ctx.trace_state.get(DD_ORIGIN)
    return origin


def _get_sampling_rate(span):
    ctx = span.get_span_context()
    return (
        span.sampler.rate
        if ctx.trace_flags.sampled
        and isinstance(span.sampler, sampling.TraceIdRatioBased)
        else None
    )


def _parse_tags_str(tags_str):
    """Parse a string of tags typically provided via environment variables.

    The expected string is of the form::
        "key1:value1,key2:value2"

    :param tags_str: A string of the above form to parse tags from.
    :return: A dict containing the tags that were parsed.
    """
    parsed_tags = {}
    if not tags_str:
        return parsed_tags

    for tag in tags_str.split(","):
        try:
            key, value = tag.split(":", 1)

            # Validate the tag
            if key == "" or value == "" or value.endswith(":"):
                raise ValueError
        except ValueError:
            logger.error(
                "Malformed tag in tag pair '%s' from tag string '%s'.",
                tag,
                tags_str,
            )
        else:
            parsed_tags[key] = value

    return parsed_tags


def _extract_tags_from_resource(resource):
    """Parse tags from resource.attributes, except service.name which
    has special significance within datadog"""
    tags = {}
    service_name = None
    if not (resource and getattr(resource, "attributes", None)):
        return [tags, service_name]

    for attribute_key, attribute_value in resource.attributes.items():
        if attribute_key == SERVICE_NAME_TAG:
            service_name = attribute_value
        else:
            tags[attribute_key] = attribute_value
    return [tags, service_name]
