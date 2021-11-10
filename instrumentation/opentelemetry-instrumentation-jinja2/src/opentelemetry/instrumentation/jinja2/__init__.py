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

Usage
-----

The OpenTelemetry ``jinja2`` integration traces templates loading, compilation
and rendering.

Usage
-----

.. code-block:: python

    from jinja2 import Environment, FileSystemLoader
    from opentelemetry.instrumentation.jinja2 import Jinja2Instrumentor


    Jinja2Instrumentor().instrument()

    env = Environment(loader=FileSystemLoader("templates"))
    template = env.get_template("mytemplate.html")

API
---
"""
# pylint: disable=no-value-for-parameter

import logging
from typing import Collection

import jinja2
from wrapt import wrap_function_wrapper as _wrap

from opentelemetry.instrumentation.instrumentor import BaseInstrumentor
from opentelemetry.instrumentation.jinja2.package import _instruments
from opentelemetry.instrumentation.jinja2.version import __version__
from opentelemetry.instrumentation.utils import unwrap
from opentelemetry.trace import SpanKind, get_tracer

logger = logging.getLogger(__name__)

ATTRIBUTE_JINJA2_TEMPLATE_NAME = "jinja2.template_name"
ATTRIBUTE_JINJA2_TEMPLATE_PATH = "jinja2.template_path"
DEFAULT_TEMPLATE_NAME = "<memory>"


def _with_tracer_wrapper(func):
    """Helper for providing tracer for wrapper functions."""

    def _with_tracer(tracer):
        def wrapper(wrapped, instance, args, kwargs):
            return func(tracer, wrapped, instance, args, kwargs)

        return wrapper

    return _with_tracer


@_with_tracer_wrapper
def _wrap_render(tracer, wrapped, instance, args, kwargs):
    """Wrap `Template.render()` or `Template.generate()`"""
    with tracer.start_as_current_span(
        "jinja2.render",
        kind=SpanKind.INTERNAL,
    ) as span:
        if span.is_recording():
            template_name = instance.name or DEFAULT_TEMPLATE_NAME
            span.set_attribute(ATTRIBUTE_JINJA2_TEMPLATE_NAME, template_name)
        return wrapped(*args, **kwargs)


@_with_tracer_wrapper
def _wrap_compile(tracer, wrapped, _, args, kwargs):
    with tracer.start_as_current_span(
        "jinja2.compile",
        kind=SpanKind.INTERNAL,
    ) as span:
        if span.is_recording():
            template_name = (
                args[1]
                if len(args) > 1
                else kwargs.get("name", DEFAULT_TEMPLATE_NAME)
            )
            span.set_attribute(ATTRIBUTE_JINJA2_TEMPLATE_NAME, template_name)
        return wrapped(*args, **kwargs)


@_with_tracer_wrapper
def _wrap_load_template(tracer, wrapped, _, args, kwargs):
    with tracer.start_as_current_span(
        "jinja2.load",
        kind=SpanKind.INTERNAL,
    ) as span:
        if span.is_recording():
            template_name = kwargs.get("name", args[0])
            span.set_attribute(ATTRIBUTE_JINJA2_TEMPLATE_NAME, template_name)
        template = None
        try:
            template = wrapped(*args, **kwargs)
            return template
        finally:
            if template and span.is_recording():
                span.set_attribute(
                    ATTRIBUTE_JINJA2_TEMPLATE_PATH, template.filename
                )


class Jinja2Instrumentor(BaseInstrumentor):
    """An instrumentor for jinja2

    See `BaseInstrumentor`
    """

    def instrumentation_dependencies(self) -> Collection[str]:
        return _instruments

    def _instrument(self, **kwargs):
        tracer_provider = kwargs.get("tracer_provider")
        tracer = get_tracer(__name__, __version__, tracer_provider)

        _wrap(jinja2, "environment.Template.render", _wrap_render(tracer))
        _wrap(jinja2, "environment.Template.generate", _wrap_render(tracer))
        _wrap(jinja2, "environment.Environment.compile", _wrap_compile(tracer))
        _wrap(
            jinja2,
            "environment.Environment._load_template",
            _wrap_load_template(tracer),
        )

    def _uninstrument(self, **kwargs):
        unwrap(jinja2.Template, "render")
        unwrap(jinja2.Template, "generate")
        unwrap(jinja2.Environment, "compile")
        unwrap(jinja2.Environment, "_load_template")
