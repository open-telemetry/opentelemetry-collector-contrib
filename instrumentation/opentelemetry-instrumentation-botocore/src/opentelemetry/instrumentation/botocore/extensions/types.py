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
from typing import Any, Dict, Optional, Tuple

from opentelemetry.trace import SpanKind
from opentelemetry.trace.span import Span
from opentelemetry.util.types import AttributeValue

_logger = logging.getLogger(__name__)

_BotoClientT = "botocore.client.BaseClient"
_BotoResultT = Dict[str, Any]
_BotoClientErrorT = "botocore.exceptions.ClientError"

_OperationParamsT = Dict[str, Any]
_AttributeMapT = Dict[str, AttributeValue]


class _AwsSdkCallContext:
    """An context object providing information about the invoked AWS service
    call.

    Args:
        service: the AWS service (e.g. s3, lambda, ...) which is called
        service_id: the name of the service in proper casing
        operation: the called operation (e.g. ListBuckets, Invoke, ...) of the
            AWS service.
        params: a dict of input parameters passed to the service operation.
        region: the AWS region in which the service call is made
        endpoint_url: the endpoint which the service operation is calling
        api_version: the API version of the called AWS service.
        span_name: the name used to create the span.
        span_kind: the kind used to create the span.
    """

    def __init__(self, client: _BotoClientT, args: Tuple[str, Dict[str, Any]]):
        operation = args[0]
        try:
            params = args[1]
        except (IndexError, TypeError):
            _logger.warning("Could not get request params.")
            params = {}

        boto_meta = client.meta
        service_model = boto_meta.service_model

        self.service = service_model.service_name.lower()  # type: str
        self.operation = operation  # type: str
        self.params = params  # type: Dict[str, Any]

        # 'operation' and 'service' are essential for instrumentation.
        # for all other attributes we extract them defensively. All of them should
        # usually exist unless some future botocore version moved things.
        self.region = self._get_attr(
            boto_meta, "region_name"
        )  # type: Optional[str]
        self.endpoint_url = self._get_attr(
            boto_meta, "endpoint_url"
        )  # type: Optional[str]

        self.api_version = self._get_attr(
            service_model, "api_version"
        )  # type: Optional[str]
        # name of the service in proper casing
        self.service_id = str(
            self._get_attr(service_model, "service_id", self.service)
        )

        self.span_name = f"{self.service_id}.{self.operation}"
        self.span_kind = SpanKind.CLIENT

    @staticmethod
    def _get_attr(obj, name: str, default=None):
        try:
            return getattr(obj, name)
        except AttributeError:
            _logger.warning("Could not get attribute '%s'", name)
            return default


class _AwsSdkExtension:
    def __init__(self, call_context: _AwsSdkCallContext):
        self._call_context = call_context

    def should_trace_service_call(self) -> bool:  # pylint:disable=no-self-use
        """Returns if the AWS SDK service call should be traced or not

        Extensions might override this function to disable tracing for certain
        operations.
        """
        return True

    def extract_attributes(self, attributes: _AttributeMapT):
        """Callback which gets invoked before the span is created.

        Extensions might override this function to extract additional attributes.
        """

    def before_service_call(self, span: Span):
        """Callback which gets invoked after the span is created but before the
        AWS SDK service is called.

        Extensions might override this function e.g. for injecting the span into
        a carrier.
        """

    def on_success(self, span: Span, result: _BotoResultT):
        """Callback that gets invoked when the AWS SDK call returns
        successfully.

        Extensions might override this function e.g. to extract and set response
        attributes on the span.
        """

    def on_error(self, span: Span, exception: _BotoClientErrorT):
        """Callback that gets invoked when the AWS SDK service call raises a
        ClientError.
        """

    def after_service_call(self):
        """Callback that gets invoked after the AWS SDK service was called.

        Extensions might override this function to do some cleanup tasks.
        """
