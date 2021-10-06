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
from logging import getLogger
from typing import Any, Callable, Collection, Dict, Optional

import wrapt
from pika.adapters import BlockingConnection
from pika.channel import Channel

from opentelemetry import trace
from opentelemetry.instrumentation.instrumentor import BaseInstrumentor
from opentelemetry.instrumentation.pika import utils
from opentelemetry.instrumentation.pika.package import _instruments
from opentelemetry.instrumentation.pika.version import __version__
from opentelemetry.instrumentation.utils import unwrap
from opentelemetry.trace import Tracer, TracerProvider

_LOG = getLogger(__name__)
_CTX_KEY = "__otel_task_span"

_FUNCTIONS_TO_UNINSTRUMENT = ["basic_publish"]


class PikaInstrumentor(BaseInstrumentor):  # type: ignore
    # pylint: disable=attribute-defined-outside-init
    @staticmethod
    def _instrument_consumers(
        consumers_dict: Dict[str, Callable[..., Any]], tracer: Tracer
    ) -> Any:
        for key, callback in consumers_dict.items():
            decorated_callback = utils._decorate_callback(
                callback, tracer, key
            )
            setattr(decorated_callback, "_original_callback", callback)
            consumers_dict[key] = decorated_callback

    @staticmethod
    def _instrument_basic_publish(channel: Channel, tracer: Tracer) -> None:
        original_function = getattr(channel, "basic_publish")
        decorated_function = utils._decorate_basic_publish(
            original_function, channel, tracer
        )
        setattr(decorated_function, "_original_function", original_function)
        channel.__setattr__("basic_publish", decorated_function)
        channel.basic_publish = decorated_function

    @staticmethod
    def _instrument_channel_functions(
        channel: Channel, tracer: Tracer
    ) -> None:
        if hasattr(channel, "basic_publish"):
            PikaInstrumentor._instrument_basic_publish(channel, tracer)

    @staticmethod
    def _uninstrument_channel_functions(channel: Channel) -> None:
        for function_name in _FUNCTIONS_TO_UNINSTRUMENT:
            if not hasattr(channel, function_name):
                continue
            function = getattr(channel, function_name)
            if hasattr(function, "_original_function"):
                channel.__setattr__(function_name, function._original_function)

    @staticmethod
    def instrument_channel(
        channel: Channel, tracer_provider: Optional[TracerProvider] = None,
    ) -> None:
        if not hasattr(channel, "_is_instrumented_by_opentelemetry"):
            channel._is_instrumented_by_opentelemetry = False
        if channel._is_instrumented_by_opentelemetry:
            _LOG.warning(
                "Attempting to instrument Pika channel while already instrumented!"
            )
            return
        tracer = trace.get_tracer(__name__, __version__, tracer_provider)
        if not hasattr(channel, "_impl"):
            _LOG.error("Could not find implementation for provided channel!")
            return
        if channel._impl._consumers:
            PikaInstrumentor._instrument_consumers(
                channel._impl._consumers, tracer
            )
        PikaInstrumentor._instrument_channel_functions(channel, tracer)

    @staticmethod
    def uninstrument_channel(channel: Channel) -> None:
        if (
            not hasattr(channel, "_is_instrumented_by_opentelemetry")
            or not channel._is_instrumented_by_opentelemetry
        ):
            _LOG.error(
                "Attempting to uninstrument Pika channel while already uninstrumented!"
            )
            return
        if not hasattr(channel, "_impl"):
            _LOG.error("Could not find implementation for provided channel!")
            return
        for key, callback in channel._impl._consumers.items():
            if hasattr(callback, "_original_callback"):
                channel._impl._consumers[key] = callback._original_callback
        PikaInstrumentor._uninstrument_channel_functions(channel)

    def _decorate_channel_function(
        self, tracer_provider: Optional[TracerProvider]
    ) -> None:
        def wrapper(wrapped, instance, args, kwargs):
            channel = wrapped(*args, **kwargs)
            self.instrument_channel(channel, tracer_provider=tracer_provider)
            return channel

        wrapt.wrap_function_wrapper(BlockingConnection, "channel", wrapper)

    def _instrument(self, **kwargs: Dict[str, Any]) -> None:
        tracer_provider: TracerProvider = kwargs.get("tracer_provider", None)
        self.__setattr__("__opentelemetry_tracer_provider", tracer_provider)
        self._decorate_channel_function(tracer_provider)

    def _uninstrument(self, **kwargs: Dict[str, Any]) -> None:
        if hasattr(self, "__opentelemetry_tracer_provider"):
            delattr(self, "__opentelemetry_tracer_provider")
        unwrap(BlockingConnection, "channel")

    def instrumentation_dependencies(self) -> Collection[str]:
        return _instruments
