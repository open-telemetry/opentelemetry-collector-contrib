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

from django.conf import settings

from opentelemetry.configuration import Configuration
from opentelemetry.instrumentation.django.middleware import _DjangoMiddleware
from opentelemetry.instrumentation.django.version import __version__
from opentelemetry.instrumentation.instrumentor import BaseInstrumentor
from opentelemetry.instrumentation.metric import (
    HTTPMetricRecorder,
    HTTPMetricType,
    MetricMixin,
)

_logger = getLogger(__name__)


class DjangoInstrumentor(BaseInstrumentor, MetricMixin):
    """An instrumentor for Django

    See `BaseInstrumentor`
    """

    _opentelemetry_middleware = ".".join(
        [_DjangoMiddleware.__module__, _DjangoMiddleware.__qualname__]
    )

    def _instrument(self, **kwargs):

        # FIXME this is probably a pattern that will show up in the rest of the
        # ext. Find a better way of implementing this.
        # FIXME Probably the evaluation of strings into boolean values can be
        # built inside the Configuration class itself with the magic method
        # __bool__

        if Configuration().DJANGO_INSTRUMENT is False:
            return

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
        self.init_metrics(
            __name__, __version__,
        )
        metric_recorder = HTTPMetricRecorder(self.meter, HTTPMetricType.SERVER)
        setattr(settings, "OTEL_METRIC_RECORDER", metric_recorder)
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
