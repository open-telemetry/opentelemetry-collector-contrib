import importlib
import logging

from opentelemetry.instrumentation.botocore.extensions.types import (
    _AwsSdkCallContext,
    _AwsSdkExtension,
)

_logger = logging.getLogger(__name__)


def _lazy_load(module, cls):
    def loader():
        imported_mod = importlib.import_module(module, __name__)
        return getattr(imported_mod, cls, None)

    return loader


_KNOWN_EXTENSIONS = {
    "sqs": _lazy_load(".sqs", "_SqsExtension"),
}


def _find_extension(call_context: _AwsSdkCallContext) -> _AwsSdkExtension:
    try:
        loader = _KNOWN_EXTENSIONS.get(call_context.service)
        if loader is None:
            return _AwsSdkExtension(call_context)

        extension_cls = loader()
        return extension_cls(call_context)
    except Exception as ex:  # pylint: disable=broad-except
        _logger.error("Error when loading extension: %s", ex)
        return _AwsSdkExtension(call_context)
