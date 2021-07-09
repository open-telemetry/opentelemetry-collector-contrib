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
Pyramid instrumentation supporting `pyramid`_, it can be enabled by
using ``PyramidInstrumentor``.

.. _pyramid: https://docs.pylonsproject.org/projects/pyramid/en/latest/

Usage
-----
There are two methods to instrument Pyramid:

Method 1 (Instrument all Configurators):
----------------------------------------

.. code:: python

    from pyramid.config import Configurator
    from opentelemetry.instrumentation.pyramid import PyramidInstrumentor

    PyramidInstrumentor().instrument()

    config = Configurator()

    # use your config as normal
    config.add_route('index', '/')

Method 2 (Instrument one Configurator):
---------------------------------------

.. code:: python

    from pyramid.config import Configurator
    from opentelemetry.instrumentation.pyramid import PyramidInstrumentor

    config = Configurator()
    PyramidInstrumentor().instrument_config(config)

    # use your config as normal
    config.add_route('index', '/')

Using ``pyramid.tweens`` setting:
---------------------------------

If you use Method 2 and then set tweens for your application with the ``pyramid.tweens`` setting,
you need to add ``opentelemetry.instrumentation.pyramid.trace_tween_factory`` explicity to the list,
*as well as* instrumenting the config as shown above.

For example:

.. code:: python

    from pyramid.config import Configurator
    from opentelemetry.instrumentation.pyramid import PyramidInstrumentor

    settings = {
        'pyramid.tweens', 'opentelemetry.instrumentation.pyramid.trace_tween_factory\\nyour_tween_no_1\\nyour_tween_no_2',
    }
    config = Configurator(settings=settings)
    PyramidInstrumentor().instrument_config(config)

    # use your config as normal.
    config.add_route('index', '/')

API
---
"""

import typing
from typing import Collection

from pyramid.config import Configurator
from pyramid.path import caller_package
from pyramid.settings import aslist
from wrapt import ObjectProxy
from wrapt import wrap_function_wrapper as _wrap

from opentelemetry.instrumentation.instrumentor import BaseInstrumentor
from opentelemetry.instrumentation.pyramid.callbacks import (
    SETTING_TRACE_ENABLED,
    TWEEN_NAME,
    trace_tween_factory,
)
from opentelemetry.instrumentation.pyramid.package import _instruments
from opentelemetry.instrumentation.pyramid.version import __version__
from opentelemetry.instrumentation.utils import unwrap
from opentelemetry.trace import TracerProvider, get_tracer


def _traced_init(wrapped, instance, args, kwargs):
    settings = kwargs.get("settings", {})
    tweens = aslist(settings.get("pyramid.tweens", []))

    if tweens and TWEEN_NAME not in settings:
        # pyramid.tweens.EXCVIEW is the name of built-in exception view provided by
        # pyramid.  We need our tween to be before it, otherwise unhandled
        # exceptions will be caught before they reach our tween.
        tweens = [TWEEN_NAME] + tweens

        settings["pyramid.tweens"] = "\n".join(tweens)

    kwargs["settings"] = settings

    # `caller_package` works by walking a fixed amount of frames up the stack
    # to find the calling package. So if we let the original `__init__`
    # function call it, our wrapper will mess things up.
    if not kwargs.get("package", None):
        # Get the package for the third frame up from this one.
        # Default is `level=2` which will give us the package from `wrapt`
        # instead of the desired package (the caller)
        kwargs["package"] = caller_package(level=3)

    wrapped(*args, **kwargs)
    instance.include("opentelemetry.instrumentation.pyramid.callbacks")


class PyramidInstrumentor(BaseInstrumentor):
    def instrumentation_dependencies(self) -> Collection[str]:
        return _instruments

    def _instrument(self, **kwargs):
        """Integrate with Pyramid Python library.
        https://docs.pylonsproject.org/projects/pyramid/en/latest/
        """
        _wrap("pyramid.config", "Configurator.__init__", _traced_init)

    def _uninstrument(self, **kwargs):
        """"Disable Pyramid instrumentation"""
        unwrap(Configurator, "__init__")

    @staticmethod
    def instrument_config(config):
        """Enable instrumentation in a Pyramid configurator.

        Args:
            config: The Configurator to instrument.
        """
        config.include("opentelemetry.instrumentation.pyramid.callbacks")

    @staticmethod
    def uninstrument_config(config):
        config.add_settings({SETTING_TRACE_ENABLED: False})
