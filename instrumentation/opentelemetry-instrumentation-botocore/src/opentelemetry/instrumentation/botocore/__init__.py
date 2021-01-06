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
Instrument `Botocore`_ to trace service requests.

There are two options for instrumenting code. The first option is to use the
``opentelemetry-instrument`` executable which will automatically
instrument your Botocore client. The second is to programmatically enable
instrumentation via the following code:

.. _Botocore: https://pypi.org/project/botocore/

Usage
-----

.. code:: python

    from opentelemetry.instrumentation.botocore import BotocoreInstrumentor
    import botocore


    # Instrument Botocore
    BotocoreInstrumentor().instrument()

    # This will create a span with Botocore-specific attributes
    session = botocore.session.get_session()
    session.set_credentials(
        access_key="access-key", secret_key="secret-key"
    )
    ec2 = self.session.create_client("ec2", region_name="us-west-2")
    ec2.describe_instances()

API
---
"""

import logging

from botocore.client import BaseClient
from botocore.exceptions import ClientError, ParamValidationError
from wrapt import ObjectProxy, wrap_function_wrapper

from opentelemetry import context as context_api
from opentelemetry import propagators
from opentelemetry.instrumentation.botocore.version import __version__
from opentelemetry.instrumentation.instrumentor import BaseInstrumentor
from opentelemetry.instrumentation.utils import unwrap
from opentelemetry.sdk.trace import Resource
from opentelemetry.trace import SpanKind, get_tracer

logger = logging.getLogger(__name__)


# pylint: disable=unused-argument
def _patched_endpoint_prepare_request(wrapped, instance, args, kwargs):
    request = args[0]
    headers = request.headers
    propagators.inject(type(headers).__setitem__, headers)
    return wrapped(*args, **kwargs)


class BotocoreInstrumentor(BaseInstrumentor):
    """An instrumentor for Botocore.

    See `BaseInstrumentor`
    """

    def _instrument(self, **kwargs):

        # pylint: disable=attribute-defined-outside-init
        self._tracer = get_tracer(
            __name__, __version__, kwargs.get("tracer_provider")
        )

        wrap_function_wrapper(
            "botocore.client",
            "BaseClient._make_api_call",
            self._patched_api_call,
        )

        wrap_function_wrapper(
            "botocore.endpoint",
            "Endpoint.prepare_request",
            _patched_endpoint_prepare_request,
        )

    def _uninstrument(self, **kwargs):
        unwrap(BaseClient, "_make_api_call")

    # pylint: disable=too-many-branches
    def _patched_api_call(self, original_func, instance, args, kwargs):
        if context_api.get_value("suppress_instrumentation"):
            return original_func(*args, **kwargs)

        # pylint: disable=protected-access
        service_name = instance._service_model.service_name
        operation_name, api_params = args

        error = None
        result = None

        with self._tracer.start_as_current_span(
            "{}".format(service_name), kind=SpanKind.CLIENT,
        ) as span:
            if span.is_recording():
                span.set_attribute("aws.operation", operation_name)
                span.set_attribute("aws.region", instance.meta.region_name)
                span.set_attribute("aws.service", service_name)
                if "QueueUrl" in api_params:
                    span.set_attribute("aws.queue_url", api_params["QueueUrl"])
                if "TableName" in api_params:
                    span.set_attribute(
                        "aws.table_name", api_params["TableName"]
                    )

            try:
                result = original_func(*args, **kwargs)
            except ClientError as ex:
                error = ex

            if error:
                result = error.response

            if span.is_recording():
                if "ResponseMetadata" in result:
                    metadata = result["ResponseMetadata"]
                    req_id = None
                    if "RequestId" in metadata:
                        req_id = metadata["RequestId"]
                    elif "HTTPHeaders" in metadata:
                        headers = metadata["HTTPHeaders"]
                        if "x-amzn-RequestId" in headers:
                            req_id = headers["x-amzn-RequestId"]
                        elif "x-amz-request-id" in headers:
                            req_id = headers["x-amz-request-id"]
                        elif "x-amz-id-2" in headers:
                            req_id = headers["x-amz-id-2"]

                    if req_id:
                        span.set_attribute(
                            "aws.request_id", req_id,
                        )

                    if "RetryAttempts" in metadata:
                        span.set_attribute(
                            "retry_attempts", metadata["RetryAttempts"],
                        )

                    if "HTTPStatusCode" in metadata:
                        span.set_attribute(
                            "http.status_code", metadata["HTTPStatusCode"],
                        )

            if error:
                raise error

            return result
