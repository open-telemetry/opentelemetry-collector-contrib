# Copyright 2020, OpenTelemetry Authors
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
The opentelemetry-instrumentation-aws-lambda package provides an `Instrumentor`
to traces calls whithin a Python AWS Lambda function.

Usage
-----

.. code:: python

    # Copy this snippet into an AWS Lambda function

    import boto3
    from opentelemetry.instrumentation.botocore import AwsBotocoreInstrumentor
    from opentelemetry.instrumentation.aws_lambda import AwsLambdaInstrumentor


    # Enable instrumentation
    AwsBotocoreInstrumentor().instrument()
    AwsLambdaInstrumentor().instrument()

    # Lambda function
    def lambda_handler(event, context):
        s3 = boto3.resource('s3')
        for bucket in s3.buckets.all():
            print(bucket.name)

        return "200 OK"

API
---

The `instrument` method accepts the following keyword args:

tracer_provider (TracerProvider) - an optional tracer provider
event_context_extractor (Callable) - a function that returns an OTel Trace
    Context given the Lambda Event the AWS Lambda was invoked with
    this function signature is: def event_context_extractor(lambda_event: Any) -> Context
for example:

.. code:: python

    from opentelemetry.instrumentation.aws_lambda import AwsLambdaInstrumentor

    def custom_event_context_extractor(lambda_event):
        # If the `TraceContextTextMapPropagator` is the global propagator, we
        # can use it to parse out the context from the HTTP Headers.
        return get_global_textmap().extract(lambda_event["foo"]["headers"])

    AwsLambdaInstrumentor().instrument(
        event_context_extractor=custom_event_context_extractor
    )
"""

import logging
import os
from importlib import import_module
from typing import Any, Callable, Collection

from wrapt import wrap_function_wrapper

from opentelemetry.context.context import Context
from opentelemetry.instrumentation.aws_lambda.package import _instruments
from opentelemetry.instrumentation.aws_lambda.version import __version__
from opentelemetry.instrumentation.instrumentor import BaseInstrumentor
from opentelemetry.instrumentation.utils import unwrap
from opentelemetry.propagate import get_global_textmap
from opentelemetry.propagators.aws.aws_xray_propagator import (
    TRACE_HEADER_KEY,
    AwsXRayPropagator,
)
from opentelemetry.semconv.resource import ResourceAttributes
from opentelemetry.semconv.trace import SpanAttributes
from opentelemetry.trace import (
    SpanKind,
    TracerProvider,
    get_tracer,
    get_tracer_provider,
)
from opentelemetry.trace.propagation import get_current_span

logger = logging.getLogger(__name__)

_HANDLER = "_HANDLER"
_X_AMZN_TRACE_ID = "_X_AMZN_TRACE_ID"
ORIG_HANDLER = "ORIG_HANDLER"
OTEL_INSTRUMENTATION_AWS_LAMBDA_FLUSH_TIMEOUT = (
    "OTEL_INSTRUMENTATION_AWS_LAMBDA_FLUSH_TIMEOUT"
)


def _default_event_context_extractor(lambda_event: Any) -> Context:
    """Default way of extracting the context from the Lambda Event.

    Assumes the Lambda Event is a map with the headers under the 'headers' key.
    This is the mapping to use when the Lambda is invoked by an API Gateway
    REST API where API Gateway is acting as a pure proxy for the request.

    See more:
    https://docs.aws.amazon.com/apigateway/latest/developerguide/set-up-lambda-proxy-integrations.html#api-gateway-simple-proxy-for-lambda-input-format

    Args:
        lambda_event: user-defined, so it could be anything, but this
            method counts on it being a map with a 'headers' key
    Returns:
        A Context with configuration found in the event.
    """
    try:
        headers = lambda_event["headers"]
    except (TypeError, KeyError):
        logger.debug(
            "Extracting context from Lambda Event failed: either enable X-Ray active tracing or configure API Gateway to trigger this Lambda function as a pure proxy. Otherwise, generated spans will have an invalid (empty) parent context."
        )
        headers = {}
    return get_global_textmap().extract(headers)


def _determine_parent_context(
    lambda_event: Any, event_context_extractor: Callable[[Any], Context]
) -> Context:
    """Determine the parent context for the current Lambda invocation.

    See more:
    https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/trace/semantic_conventions/instrumentation/aws-lambda.md#determining-the-parent-of-a-span

    Args:
        lambda_event: user-defined, so it could be anything, but this
            method counts it being a map with a 'headers' key
    Returns:
        A Context with configuration found in the carrier.
    """
    parent_context = None

    xray_env_var = os.environ.get(_X_AMZN_TRACE_ID)

    if xray_env_var:
        parent_context = AwsXRayPropagator().extract(
            {TRACE_HEADER_KEY: xray_env_var}
        )

    if (
        parent_context
        and get_current_span(parent_context)
        .get_span_context()
        .trace_flags.sampled
    ):
        return parent_context

    if event_context_extractor:
        parent_context = event_context_extractor(lambda_event)
    else:
        parent_context = _default_event_context_extractor(lambda_event)

    return parent_context


def _instrument(
    wrapped_module_name,
    wrapped_function_name,
    flush_timeout,
    event_context_extractor: Callable[[Any], Context],
    tracer_provider: TracerProvider = None,
):
    def _instrumented_lambda_handler_call(
        call_wrapped, instance, args, kwargs
    ):
        orig_handler_name = ".".join(
            [wrapped_module_name, wrapped_function_name]
        )

        lambda_event = args[0]

        parent_context = _determine_parent_context(
            lambda_event, event_context_extractor
        )

        span_kind = None
        try:
            if lambda_event["Records"][0]["eventSource"] in set(
                ["aws:sqs", "aws:s3", "aws:sns", "aws:dynamodb"]
            ):
                # See more:
                # https://docs.aws.amazon.com/lambda/latest/dg/with-sqs.html
                # https://docs.aws.amazon.com/lambda/latest/dg/with-sns.html
                # https://docs.aws.amazon.com/AmazonS3/latest/userguide/notification-content-structure.html
                # https://docs.aws.amazon.com/lambda/latest/dg/with-ddb.html
                span_kind = SpanKind.CONSUMER
            else:
                span_kind = SpanKind.SERVER
        except (IndexError, KeyError, TypeError):
            span_kind = SpanKind.SERVER

        tracer = get_tracer(__name__, __version__, tracer_provider)

        with tracer.start_as_current_span(
            name=orig_handler_name,
            context=parent_context,
            kind=span_kind,
        ) as span:
            if span.is_recording():
                lambda_context = args[1]
                # NOTE: The specs mention an exception here, allowing the
                # `ResourceAttributes.FAAS_ID` attribute to be set as a span
                # attribute instead of a resource attribute.
                #
                # See more:
                # https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/trace/semantic_conventions/faas.md#example
                span.set_attribute(
                    ResourceAttributes.FAAS_ID,
                    lambda_context.invoked_function_arn,
                )
                span.set_attribute(
                    SpanAttributes.FAAS_EXECUTION,
                    lambda_context.aws_request_id,
                )

            result = call_wrapped(*args, **kwargs)

        _tracer_provider = tracer_provider or get_tracer_provider()
        try:
            # NOTE: `force_flush` before function quit in case of Lambda freeze.
            # Assumes we are using the OpenTelemetry SDK implementation of the
            # `TracerProvider`.
            _tracer_provider.force_flush(flush_timeout)
        except Exception:  # pylint: disable=broad-except
            logger.error(
                "TracerProvider was missing `force_flush` method. This is necessary in case of a Lambda freeze and would exist in the OTel SDK implementation."
            )

        return result

    wrap_function_wrapper(
        wrapped_module_name,
        wrapped_function_name,
        _instrumented_lambda_handler_call,
    )


class AwsLambdaInstrumentor(BaseInstrumentor):
    def instrumentation_dependencies(self) -> Collection[str]:
        return _instruments

    def _instrument(self, **kwargs):
        """Instruments Lambda Handlers on AWS Lambda.

        See more:
        https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/trace/semantic_conventions/instrumentation/aws-lambda.md#instrumenting-aws-lambda

        Args:
            **kwargs: Optional arguments
                ``tracer_provider``: a TracerProvider, defaults to global
                ``event_context_extractor``: a method which takes the Lambda
                    Event as input and extracts an OTel Context from it. By default,
                    the context is extracted from the HTTP headers of an API Gateway
                    request.
        """
        lambda_handler = os.environ.get(ORIG_HANDLER, os.environ.get(_HANDLER))
        # pylint: disable=attribute-defined-outside-init
        (
            self._wrapped_module_name,
            self._wrapped_function_name,
        ) = lambda_handler.rsplit(".", 1)

        flush_timeout_env = os.environ.get(
            OTEL_INSTRUMENTATION_AWS_LAMBDA_FLUSH_TIMEOUT, ""
        )
        flush_timeout = 30000
        try:
            flush_timeout = int(flush_timeout_env)
        except ValueError:
            logger.warning(
                "Could not convert OTEL_INSTRUMENTATION_AWS_LAMBDA_FLUSH_TIMEOUT value %s to int",
                flush_timeout_env,
            )

        _instrument(
            self._wrapped_module_name,
            self._wrapped_function_name,
            flush_timeout,
            event_context_extractor=kwargs.get(
                "event_context_extractor", _default_event_context_extractor
            ),
            tracer_provider=kwargs.get("tracer_provider"),
        )

    def _uninstrument(self, **kwargs):
        unwrap(
            import_module(self._wrapped_module_name),
            self._wrapped_function_name,
        )
