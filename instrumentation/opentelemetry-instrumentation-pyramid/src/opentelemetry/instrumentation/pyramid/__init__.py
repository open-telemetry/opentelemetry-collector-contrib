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
you need to explicitly add ``opentelemetry.instrumentation.pyramid.trace_tween_factory`` to the list,
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

Configuration
-------------

Exclude lists
*************
To exclude certain URLs from tracking, set the environment variable ``OTEL_PYTHON_PYRAMID_EXCLUDED_URLS``
(or ``OTEL_PYTHON_EXCLUDED_URLS`` to cover all instrumentations) to a string of comma delimited regexes that match the
URLs.

For example,

::

    export OTEL_PYTHON_PYRAMID_EXCLUDED_URLS="client/.*/info,healthcheck"

will exclude requests such as ``https://site/client/123/info`` and ``https://site/xyz/healthcheck``.

Capture HTTP request and response headers
*****************************************
You can configure the agent to capture specified HTTP headers as span attributes, according to the
`semantic convention <https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/trace/semantic_conventions/http.md#http-request-and-response-headers>`_.

Request headers
***************
To capture HTTP request headers as span attributes, set the environment variable
``OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SERVER_REQUEST`` to a comma delimited list of HTTP header names.

For example,
::

    export OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SERVER_REQUEST="content-type,custom_request_header"

will extract ``content-type`` and ``custom_request_header`` from the request headers and add them as span attributes.

Request header names in Pyramid are case-insensitive and ``-`` characters are replaced by ``_``. So, giving the header
name as ``CUStom_Header`` in the environment variable will capture the header named ``custom-header``.

Regular expressions may also be used to match multiple headers that correspond to the given pattern.  For example:
::

    export OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SERVER_REQUEST="Accept.*,X-.*"

Would match all request headers that start with ``Accept`` and ``X-``.

To capture all request headers, set ``OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SERVER_REQUEST`` to ``".*"``.
::

    export OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SERVER_REQUEST=".*"

The name of the added span attribute will follow the format ``http.request.header.<header_name>`` where ``<header_name>``
is the normalized HTTP header name (lowercase, with ``-`` replaced by ``_``). The value of the attribute will be a
single item list containing all the header values.

For example:
``http.request.header.custom_request_header = ["<value1>,<value2>"]``

Response headers
****************
To capture HTTP response headers as span attributes, set the environment variable
``OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SERVER_RESPONSE`` to a comma delimited list of HTTP header names.

For example,
::

    export OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SERVER_RESPONSE="content-type,custom_response_header"

will extract ``content-type`` and ``custom_response_header`` from the response headers and add them as span attributes.

Response header names in Pyramid are case-insensitive. So, giving the header name as ``CUStom-Header`` in the environment
variable will capture the header named ``custom-header``.

Regular expressions may also be used to match multiple headers that correspond to the given pattern.  For example:
::

    export OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SERVER_RESPONSE="Content.*,X-.*"

Would match all response headers that start with ``Content`` and ``X-``.

To capture all response headers, set ``OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SERVER_RESPONSE`` to ``".*"``.
::

    export OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SERVER_RESPONSE=".*"

The name of the added span attribute will follow the format ``http.response.header.<header_name>`` where ``<header_name>``
is the normalized HTTP header name (lowercase, with ``-`` replaced by ``_``). The value of the attribute will be a
single item list containing all the header values.

For example:
``http.response.header.custom_response_header = ["<value1>,<value2>"]``

Sanitizing headers
******************
In order to prevent storing sensitive data such as personally identifiable information (PII), session keys, passwords,
etc, set the environment variable ``OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SANITIZE_FIELDS``
to a comma delimited list of HTTP header names to be sanitized.  Regexes may be used, and all header names will be
matched in a case-insensitive manner.

For example,
::

    export OTEL_INSTRUMENTATION_HTTP_CAPTURE_HEADERS_SANITIZE_FIELDS=".*session.*,set-cookie"

will replace the value of headers such as ``session-id`` and ``set-cookie`` with ``[REDACTED]`` in the span.

Note:
    The environment variable names used to capture HTTP headers are still experimental, and thus are subject to change.

API
---
"""
import platform
from typing import Collection

from pyramid.config import Configurator
from pyramid.path import caller_package
from pyramid.settings import aslist
from wrapt import wrap_function_wrapper as _wrap

from opentelemetry.instrumentation.instrumentor import BaseInstrumentor
from opentelemetry.instrumentation.pyramid.callbacks import (
    SETTING_TRACE_ENABLED,
    TWEEN_NAME,
    trace_tween_factory,
)
from opentelemetry.instrumentation.pyramid.package import _instruments
from opentelemetry.instrumentation.utils import unwrap

# test_automatic.TestAutomatic.test_tween_list needs trace_tween_factory to be
# imported in this module. The next line is necessary to avoid a lint error
# from importing an unused symbol.
trace_tween_factory  # pylint: disable=pointless-statement

if platform.python_implementation() == "PyPy":
    CALLER_LEVELS = 3
else:
    CALLER_LEVELS = 2


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
        # Get the package for the 2nd frame up from this one.
        # Default is `level=2` one level down (in Configurator.__init__).
        # We want the 3rd level from _there_. Since we are already 1 level above,
        # we need the 2nd level up from here, which will give us the package from
        # `wrapt` instead of the desired package (the caller)
        kwargs["package"] = caller_package(level=CALLER_LEVELS)

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
        """ "Disable Pyramid instrumentation"""
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
