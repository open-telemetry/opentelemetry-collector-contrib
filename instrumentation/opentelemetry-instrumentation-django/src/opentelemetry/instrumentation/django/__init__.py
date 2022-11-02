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

Instrument `django`_ to trace Django applications.

.. _django: https://pypi.org/project/django/

SQLCOMMENTER
*****************************************
You can optionally configure Django instrumentation to enable sqlcommenter which enriches
the query with contextual information.

Usage
-----

.. code:: python

    from opentelemetry.instrumentation.django import DjangoInstrumentor

    DjangoInstrumentor().instrument(is_sql_commentor_enabled=True)


For example,
::

   Invoking Users().objects.all() will lead to sql query "select * from auth_users" but when SQLCommenter is enabled
   the query will get appended with some configurable tags like "select * from auth_users /*metrics=value*/;"


SQLCommenter Configurations
***************************
We can configure the tags to be appended to the sqlquery log by adding below variables to the settings.py

SQLCOMMENTER_WITH_FRAMEWORK = True(Default) or False

For example,
::
Enabling this flag will add django framework and it's version which is /*framework='django%3A2.2.3*/

SQLCOMMENTER_WITH_CONTROLLER = True(Default) or False

For example,
::
Enabling this flag will add controller name that handles the request /*controller='index'*/

SQLCOMMENTER_WITH_ROUTE = True(Default) or False

For example,
::
Enabling this flag will add url path that handles the request /*route='polls/'*/

SQLCOMMENTER_WITH_APP_NAME = True(Default) or False

For example,
::
Enabling this flag will add app name that handles the request /*app_name='polls'*/

SQLCOMMENTER_WITH_OPENTELEMETRY = True(Default) or False

For example,
::
Enabling this flag will add opentelemetry traceparent /*traceparent='00-fd720cffceba94bbf75940ff3caaf3cc-4fd1a2bdacf56388-01'*/

SQLCOMMENTER_WITH_DB_DRIVER = True(Default) or False

For example,
::
Enabling this flag will add name of the db driver /*db_driver='django.db.backends.postgresql'*/

Usage
-----

.. code:: python

    from opentelemetry.instrumentation.django import DjangoInstrumentor

    DjangoInstrumentor().instrument()


Configuration
-------------

Exclude lists
*************
To exclude certain URLs from tracking, set the environment variable ``OTEL_PYTHON_DJANGO_EXCLUDED_URLS``
(or ``OTEL_PYTHON_EXCLUDED_URLS`` to cover all instrumentations) to a string of comma delimited regexes that match the
URLs.

For example,

::

    export OTEL_PYTHON_DJANGO_EXCLUDED_URLS="client/.*/info,healthcheck"

will exclude requests such as ``https://site/client/123/info`` and ``https://site/xyz/healthcheck``.

Request attributes
********************
To extract attributes from Django's request object and use them as span attributes, set the environment variable
``OTEL_PYTHON_DJANGO_TRACED_REQUEST_ATTRS`` to a comma delimited list of request attribute names.

For example,

::

    export OTEL_PYTHON_DJANGO_TRACED_REQUEST_ATTRS='path_info,content_type'

will extract the ``path_info`` and ``content_type`` attributes from every traced request and add them as span attributes.

Django Request object reference: https://docs.djangoproject.com/en/3.1/ref/request-response/#attributes

Request and Response hooks
***************************
This instrumentation supports request and response hooks. These are functions that get called
right after a span is created for a request and right before the span is finished for the response.
The hooks can be configured as follows:

.. code:: python

    def request_hook(span, request):
        pass

    def response_hook(span, request, response):
        pass

    DjangoInstrumentation().instrument(request_hook=request_hook, response_hook=response_hook)

Django Request object: https://docs.djangoproject.com/en/3.1/ref/request-response/#httprequest-objects
Django Response object: https://docs.djangoproject.com/en/3.1/ref/request-response/#httpresponse-objects

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

Request header names in Django are case-insensitive. So, giving the header name as ``CUStom-Header`` in the environment
variable will capture the header named ``custom-header``.

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

Response header names in Django are case-insensitive. So, giving the header name as ``CUStom-Header`` in the environment
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

from logging import getLogger
from os import environ
from typing import Collection

from django import VERSION as django_version
from django.conf import settings
from django.core.exceptions import ImproperlyConfigured

from opentelemetry.instrumentation.django.environment_variables import (
    OTEL_PYTHON_DJANGO_INSTRUMENT,
)
from opentelemetry.instrumentation.django.middleware.otel_middleware import (
    _DjangoMiddleware,
)
from opentelemetry.instrumentation.django.package import _instruments
from opentelemetry.instrumentation.django.version import __version__
from opentelemetry.instrumentation.instrumentor import BaseInstrumentor
from opentelemetry.metrics import get_meter
from opentelemetry.semconv.metrics import MetricInstruments
from opentelemetry.trace import get_tracer

DJANGO_2_0 = django_version >= (2, 0)

_logger = getLogger(__name__)


def _get_django_middleware_setting() -> str:
    # In Django versions 1.x, setting MIDDLEWARE_CLASSES can be used as a legacy
    # alternative to MIDDLEWARE. This is the case when `settings.MIDDLEWARE` has
    # its default value (`None`).
    if not DJANGO_2_0 and getattr(settings, "MIDDLEWARE", None) is None:
        return "MIDDLEWARE_CLASSES"
    return "MIDDLEWARE"


class DjangoInstrumentor(BaseInstrumentor):
    """An instrumentor for Django

    See `BaseInstrumentor`
    """

    _opentelemetry_middleware = ".".join(
        [_DjangoMiddleware.__module__, _DjangoMiddleware.__qualname__]
    )

    _sql_commenter_middleware = "opentelemetry.instrumentation.django.middleware.sqlcommenter_middleware.SqlCommenter"

    def instrumentation_dependencies(self) -> Collection[str]:
        return _instruments

    def _instrument(self, **kwargs):

        # FIXME this is probably a pattern that will show up in the rest of the
        # ext. Find a better way of implementing this.
        if environ.get(OTEL_PYTHON_DJANGO_INSTRUMENT) == "False":
            return

        tracer_provider = kwargs.get("tracer_provider")
        meter_provider = kwargs.get("meter_provider")
        tracer = get_tracer(
            __name__,
            __version__,
            tracer_provider=tracer_provider,
        )
        meter = get_meter(__name__, __version__, meter_provider=meter_provider)
        _DjangoMiddleware._tracer = tracer
        _DjangoMiddleware._meter = meter
        _DjangoMiddleware._otel_request_hook = kwargs.pop("request_hook", None)
        _DjangoMiddleware._otel_response_hook = kwargs.pop(
            "response_hook", None
        )
        _DjangoMiddleware._duration_histogram = meter.create_histogram(
            name=MetricInstruments.HTTP_SERVER_DURATION,
            unit="ms",
            description="measures the duration of the inbound http request",
        )
        _DjangoMiddleware._active_request_counter = meter.create_up_down_counter(
            name=MetricInstruments.HTTP_SERVER_ACTIVE_REQUESTS,
            unit="requests",
            description="measures the number of concurrent HTTP requests those are currently in flight",
        )
        # This can not be solved, but is an inherent problem of this approach:
        # the order of middleware entries matters, and here you have no control
        # on that:
        # https://docs.djangoproject.com/en/3.0/topics/http/middleware/#activating-middleware
        # https://docs.djangoproject.com/en/3.0/ref/middleware/#middleware-ordering

        _middleware_setting = _get_django_middleware_setting()
        settings_middleware = []
        try:
            settings_middleware = getattr(settings, _middleware_setting, [])
        except ImproperlyConfigured as exception:
            _logger.debug(
                "DJANGO_SETTINGS_MODULE environment variable not configured. Defaulting to empty settings: %s",
                exception,
            )
            settings.configure()
            settings_middleware = getattr(settings, _middleware_setting, [])
        except ModuleNotFoundError as exception:
            _logger.debug(
                "DJANGO_SETTINGS_MODULE points to a non-existent module. Defaulting to empty settings: %s",
                exception,
            )
            settings.configure()
            settings_middleware = getattr(settings, _middleware_setting, [])

        # Django allows to specify middlewares as a tuple, so we convert this tuple to a
        # list, otherwise we wouldn't be able to call append/remove
        if isinstance(settings_middleware, tuple):
            settings_middleware = list(settings_middleware)

        is_sql_commentor_enabled = kwargs.pop("is_sql_commentor_enabled", None)

        if is_sql_commentor_enabled:
            settings_middleware.insert(0, self._sql_commenter_middleware)

        settings_middleware.insert(0, self._opentelemetry_middleware)

        setattr(settings, _middleware_setting, settings_middleware)

    def _uninstrument(self, **kwargs):
        _middleware_setting = _get_django_middleware_setting()
        settings_middleware = getattr(settings, _middleware_setting, None)

        # FIXME This is starting to smell like trouble. We have 2 mechanisms
        # that may make this condition be True, one implemented in
        # BaseInstrumentor and another one implemented in _instrument. Both
        # stop _instrument from running and thus, settings_middleware not being
        # set.
        if settings_middleware is None or (
            self._opentelemetry_middleware not in settings_middleware
        ):
            return

        settings_middleware.remove(self._opentelemetry_middleware)
        setattr(settings, _middleware_setting, settings_middleware)
