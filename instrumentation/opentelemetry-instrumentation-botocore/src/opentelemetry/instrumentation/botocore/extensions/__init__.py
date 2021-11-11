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
    "dynamodb": _lazy_load(".dynamodb", "_DynamoDbExtension"),
    "lambda": _lazy_load(".lmbd", "_LambdaExtension"),
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
