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
Instrument `Boto`_ to trace service requests.

There are two options for instrumenting code. The first option is to use the
``opentelemetry-instrument`` executable which will automatically
instrument your Boto client. The second is to programmatically enable
instrumentation via the following code:

.. _boto: https://pypi.org/project/boto/

Usage
-----

.. code:: python

    from opentelemetry.instrumentation.boto import BotoInstrumentor
    import boto


    # Instrument Boto
    BotoInstrumentor().instrument()

    # This will create a span with Boto-specific attributes
    ec2 = boto.ec2.connect_to_region("us-west-2")
    ec2.get_all_instances()

API
---
"""

import logging
from inspect import currentframe
from typing import Collection

from boto.connection import AWSAuthConnection, AWSQueryConnection
from wrapt import wrap_function_wrapper

from opentelemetry.instrumentation.boto.package import _instruments
from opentelemetry.instrumentation.boto.version import __version__
from opentelemetry.instrumentation.instrumentor import BaseInstrumentor
from opentelemetry.instrumentation.utils import unwrap
from opentelemetry.sdk.trace import Resource
from opentelemetry.semconv.trace import SpanAttributes
from opentelemetry.trace import SpanKind, get_tracer

logger = logging.getLogger(__name__)

SERVICE_PARAMS_BLOCK_LIST = {"s3": ["params.Body"]}


def _get_instance_region_name(instance):
    region = getattr(instance, "region", None)

    if not region:
        return None
    if isinstance(region, str):
        return region.split(":")[1]
    return region.name


class BotoInstrumentor(BaseInstrumentor):
    """A instrumentor for Boto

    See `BaseInstrumentor`
    """

    def __init__(self):
        super().__init__()
        self._original_boto = None

    def instrumentation_dependencies(self) -> Collection[str]:
        return _instruments

    def _instrument(self, **kwargs):
        # AWSQueryConnection and AWSAuthConnection are two different classes
        # called by different services for connection.
        # For exemple EC2 uses AWSQueryConnection and S3 uses
        # AWSAuthConnection

        # pylint: disable=attribute-defined-outside-init
        self._tracer = get_tracer(
            __name__, __version__, kwargs.get("tracer_provider")
        )

        wrap_function_wrapper(
            "boto.connection",
            "AWSQueryConnection.make_request",
            self._patched_query_request,
        )
        wrap_function_wrapper(
            "boto.connection",
            "AWSAuthConnection.make_request",
            self._patched_auth_request,
        )

    def _uninstrument(self, **kwargs):
        unwrap(AWSQueryConnection, "make_request")
        unwrap(AWSAuthConnection, "make_request")

    def _common_request(  # pylint: disable=too-many-locals
        self,
        args_name,
        traced_args,
        operation_name,
        original_func,
        instance,
        args,
        kwargs,
    ):

        endpoint_name = getattr(instance, "host").split(".")[0]

        with self._tracer.start_as_current_span(
            "{}.command".format(endpoint_name), kind=SpanKind.CONSUMER,
        ) as span:
            span.set_attribute("endpoint", endpoint_name)
            if args:
                http_method = args[0]
                span.set_attribute("http_method", http_method.lower())

            # Original func returns a boto.connection.HTTPResponse object
            result = original_func(*args, **kwargs)

            if span.is_recording():
                add_span_arg_tags(
                    span, endpoint_name, args, args_name, traced_args,
                )

                # Obtaining region name
                region_name = _get_instance_region_name(instance)

                meta = {
                    "aws.agent": "boto",
                    "aws.operation": operation_name,
                }
                if region_name:
                    meta["aws.region"] = region_name

                for key, value in meta.items():
                    span.set_attribute(key, value)

                span.set_attribute(
                    SpanAttributes.HTTP_STATUS_CODE, getattr(result, "status")
                )
                span.set_attribute(
                    SpanAttributes.HTTP_METHOD, getattr(result, "_method")
                )

            return result

    def _patched_query_request(self, original_func, instance, args, kwargs):

        return self._common_request(
            ("operation_name", "params", "path", "verb"),
            ["operation_name", "params", "path"],
            args[0] if args else None,
            original_func,
            instance,
            args,
            kwargs,
        )

    def _patched_auth_request(self, original_func, instance, args, kwargs):
        operation_name = None

        frame = currentframe().f_back
        operation_name = None
        while frame:
            if frame.f_code.co_name == "make_request":
                operation_name = frame.f_back.f_code.co_name
                break
            frame = frame.f_back

        return self._common_request(
            (
                "method",
                "path",
                "headers",
                "data",
                "host",
                "auth_path",
                "sender",
            ),
            ["path", "data", "host"],
            operation_name,
            original_func,
            instance,
            args,
            kwargs,
        )


def flatten_dict(dict_, sep=".", prefix=""):
    """
    Returns a normalized dict of depth 1 with keys in order of embedding
    """
    # NOTE: This should probably be in `opentelemetry.instrumentation.utils`.
    # adapted from https://stackoverflow.com/a/19647596
    return (
        {
            prefix + sep + k if prefix else k: v
            for kk, vv in dict_.items()
            for k, v in flatten_dict(vv, sep, kk).items()
        }
        if isinstance(dict_, dict)
        else {prefix: dict_}
    )


def add_span_arg_tags(span, aws_service, args, args_names, args_traced):
    def truncate_arg_value(value, max_len=1024):
        """Truncate values which are bytes and greater than `max_len`.
        Useful for parameters like "Body" in `put_object` operations.
        """
        if isinstance(value, bytes) and len(value) > max_len:
            return b"..."

        return value

    if not span.is_recording():
        return

    # Do not trace `Key Management Service` or `Secure Token Service` API calls
    # over concerns of security leaks.
    if aws_service not in {"kms", "sts"}:
        tags = dict(
            (name, value)
            for (name, value) in zip(args_names, args)
            if name in args_traced
        )
        tags = flatten_dict(tags)

        for param_key, value in tags.items():
            if param_key in SERVICE_PARAMS_BLOCK_LIST.get(aws_service, {}):
                continue

            span.set_attribute(param_key, truncate_arg_value(value))
