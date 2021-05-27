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

Usage
-----

.. code:: python

    from opentelemetry.instrumentation.django import DjangoInstrumentor

    DjangoInstrumentor().instrument()


Configuration
-------------

Exclude lists
*************
To exclude certain URLs from being tracked, set the environment variable ``OTEL_PYTHON_DJANGO_EXCLUDED_URLS`` with comma delimited regexes representing which URLs to exclude.

For example,

::

    export OTEL_PYTHON_DJANGO_EXCLUDED_URLS="client/.*/info,healthcheck"

will exclude requests such as ``https://site/client/123/info`` and ``https://site/xyz/healthcheck``.

Request attributes
********************
To extract certain attributes from Django's request object and use them as span attributes, set the environment variable ``OTEL_PYTHON_DJANGO_TRACED_REQUEST_ATTRS`` to a comma
delimited list of request attribute names.

For example,

::

    export OTEL_PYTHON_DJANGO_TRACED_REQUEST_ATTRS='path_info,content_type'

will extract path_info and content_type attributes from every traced request and add them as span attritbues.

Django Request object reference: https://docs.djangoproject.com/en/3.1/ref/request-response/#attributes

Request and Response hooks
***************************
The instrumentation supports specifying request and response hooks. These are functions that get called back by the instrumentation right after a Span is created for a request
and right before the span is finished while processing a response. The hooks can be configured as follows:

.. code:: python

    def request_hook(span, request):
        pass

    def response_hook(span, request, response):
        pass

    DjangoInstrumentation().instrument(request_hook=request_hook, response_hook=response_hook)
"""

from logging import getLogger
from os import environ
from typing import Collection

from django.conf import settings

from opentelemetry.instrumentation.django.environment_variables import (
    OTEL_PYTHON_DJANGO_INSTRUMENT,
)
from opentelemetry.instrumentation.django.middleware import _DjangoMiddleware
from opentelemetry.instrumentation.django.package import _instruments
from opentelemetry.instrumentation.django.version import __version__
from opentelemetry.instrumentation.instrumentor import BaseInstrumentor
from opentelemetry.trace import get_tracer

_logger = getLogger(__name__)


class DjangoInstrumentor(BaseInstrumentor):
    """An instrumentor for Django

    See `BaseInstrumentor`
    """

    _opentelemetry_middleware = ".".join(
        [_DjangoMiddleware.__module__, _DjangoMiddleware.__qualname__]
    )

    def instrumentation_dependencies(self) -> Collection[str]:
        return _instruments

    def _instrument(self, **kwargs):

        # FIXME this is probably a pattern that will show up in the rest of the
        # ext. Find a better way of implementing this.
        if environ.get(OTEL_PYTHON_DJANGO_INSTRUMENT) == "False":
            return

        tracer_provider = kwargs.get("tracer_provider")
        tracer = get_tracer(
            __name__, __version__, tracer_provider=tracer_provider,
        )

        _DjangoMiddleware._tracer = tracer

        _DjangoMiddleware._otel_request_hook = kwargs.pop("request_hook", None)
        _DjangoMiddleware._otel_response_hook = kwargs.pop(
            "response_hook", None
        )

        # This can not be solved, but is an inherent problem of this approach:
        # the order of middleware entries matters, and here you have no control
        # on that:
        # https://docs.djangoproject.com/en/3.0/topics/http/middleware/#activating-middleware
        # https://docs.djangoproject.com/en/3.0/ref/middleware/#middleware-ordering

        settings_middleware = getattr(settings, "MIDDLEWARE", [])
        # Django allows to specify middlewares as a tuple, so we convert this tuple to a
        # list, otherwise we wouldn't be able to call append/remove
        if isinstance(settings_middleware, tuple):
            settings_middleware = list(settings_middleware)

        settings_middleware.insert(0, self._opentelemetry_middleware)
        setattr(settings, "MIDDLEWARE", settings_middleware)

    def _uninstrument(self, **kwargs):
        settings_middleware = getattr(settings, "MIDDLEWARE", None)

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
        setattr(settings, "MIDDLEWARE", settings_middleware)
