import logging
from typing import Any, Dict, Optional, Tuple

from opentelemetry.trace import SpanKind

_logger = logging.getLogger(__name__)

_BotoClientT = "botocore.client.BaseClient"

_OperationParamsT = Dict[str, Any]


class _AwsSdkCallContext:
    """An context object providing information about the invoked AWS service
    call.

    Args:
        service: the AWS service (e.g. s3, lambda, ...) which is called
        service_id: the name of the service in propper casing
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
