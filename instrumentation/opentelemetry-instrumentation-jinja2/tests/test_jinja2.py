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

import os
from unittest import mock

import jinja2

from opentelemetry import trace as trace_api
from opentelemetry.instrumentation.jinja2 import Jinja2Instrumentor
from opentelemetry.test.test_base import TestBase
from opentelemetry.trace import get_tracer

TEST_DIR = os.path.dirname(os.path.realpath(__file__))
TMPL_DIR = os.path.join(TEST_DIR, "templates")


class TestJinja2Instrumentor(TestBase):
    def setUp(self):
        super().setUp()
        Jinja2Instrumentor().instrument()
        # prevent cache effects when using Template('code...')
        # pylint: disable=protected-access
        jinja2.environment._spontaneous_environments.clear()
        self.tracer = get_tracer(__name__)

    def tearDown(self):
        super().tearDown()
        Jinja2Instrumentor().uninstrument()

    def test_render_inline_template_with_root(self):
        with self.tracer.start_as_current_span("root"):
            template = jinja2.environment.Template("Hello {{name}}!")
            self.assertEqual(template.render(name="Jinja"), "Hello Jinja!")

        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 3)

        # pylint:disable=unbalanced-tuple-unpacking
        render, template, root = spans[:3]

        self.assertIs(render.parent, root.get_span_context())
        self.assertIs(template.parent, root.get_span_context())
        self.assertIsNone(root.parent)

    def test_render_not_recording(self):
        mock_tracer = mock.Mock()
        mock_span = mock.Mock()
        mock_span.is_recording.return_value = False
        mock_tracer.start_span.return_value = mock_span
        with mock.patch("opentelemetry.trace.get_tracer") as tracer:
            tracer.return_value = mock_tracer
            jinja2.environment.Template("Hello {{name}}!")
            self.assertFalse(mock_span.is_recording())
            self.assertTrue(mock_span.is_recording.called)
            self.assertFalse(mock_span.set_attribute.called)
            self.assertFalse(mock_span.set_status.called)

    def test_render_inline_template(self):
        template = jinja2.environment.Template("Hello {{name}}!")
        self.assertEqual(template.render(name="Jinja"), "Hello Jinja!")

        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 2)

        # pylint:disable=unbalanced-tuple-unpacking
        template, render = spans

        self.assertEqual(template.name, "jinja2.compile")
        self.assertIs(template.kind, trace_api.SpanKind.INTERNAL)
        self.assertEqual(
            template.attributes, {"jinja2.template_name": "<memory>"},
        )

        self.assertEqual(render.name, "jinja2.render")
        self.assertIs(render.kind, trace_api.SpanKind.INTERNAL)
        self.assertEqual(
            render.attributes, {"jinja2.template_name": "<memory>"},
        )

    def test_generate_inline_template_with_root(self):
        with self.tracer.start_as_current_span("root"):
            template = jinja2.environment.Template("Hello {{name}}!")
            self.assertEqual(
                "".join(template.generate(name="Jinja")), "Hello Jinja!"
            )

        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 3)

        # pylint:disable=unbalanced-tuple-unpacking
        template, generate, root = spans

        self.assertIs(generate.parent, root.get_span_context())
        self.assertIs(template.parent, root.get_span_context())
        self.assertIsNone(root.parent)

    def test_generate_inline_template(self):
        template = jinja2.environment.Template("Hello {{name}}!")
        self.assertEqual(
            "".join(template.generate(name="Jinja")), "Hello Jinja!"
        )

        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 2)

        # pylint:disable=unbalanced-tuple-unpacking
        template, generate = spans[:2]

        self.assertEqual(template.name, "jinja2.compile")
        self.assertIs(template.kind, trace_api.SpanKind.INTERNAL)
        self.assertEqual(
            template.attributes, {"jinja2.template_name": "<memory>"},
        )

        self.assertEqual(generate.name, "jinja2.render")
        self.assertIs(generate.kind, trace_api.SpanKind.INTERNAL)
        self.assertEqual(
            generate.attributes, {"jinja2.template_name": "<memory>"},
        )

    def test_file_template_with_root(self):
        with self.tracer.start_as_current_span("root"):
            loader = jinja2.loaders.FileSystemLoader(TMPL_DIR)
            env = jinja2.Environment(loader=loader)
            template = env.get_template("template.html")
            self.assertEqual(
                template.render(name="Jinja"), "Message: Hello Jinja!"
            )

        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 6)

        # pylint:disable=unbalanced-tuple-unpacking
        compile2, load2, compile1, load1, render, root = spans

        self.assertIs(compile2.parent, load2.get_span_context())
        self.assertIs(load2.parent, root.get_span_context())
        self.assertIs(compile1.parent, load1.get_span_context())
        self.assertIs(load1.parent, render.get_span_context())
        self.assertIs(render.parent, root.get_span_context())
        self.assertIsNone(root.parent)

    def test_file_template(self):
        loader = jinja2.loaders.FileSystemLoader(TMPL_DIR)
        env = jinja2.Environment(loader=loader)
        template = env.get_template("template.html")
        self.assertEqual(
            template.render(name="Jinja"), "Message: Hello Jinja!"
        )

        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 5)

        # pylint:disable=unbalanced-tuple-unpacking
        compile2, load2, compile1, load1, render = spans

        self.assertEqual(compile2.name, "jinja2.compile")
        self.assertEqual(load2.name, "jinja2.load")
        self.assertEqual(compile1.name, "jinja2.compile")
        self.assertEqual(load1.name, "jinja2.load")
        self.assertEqual(render.name, "jinja2.render")

        self.assertEqual(
            compile2.attributes, {"jinja2.template_name": "template.html"},
        )
        self.assertEqual(
            load2.attributes,
            {
                "jinja2.template_name": "template.html",
                "jinja2.template_path": os.path.join(
                    TMPL_DIR, "template.html"
                ),
            },
        )
        self.assertEqual(
            compile1.attributes, {"jinja2.template_name": "base.html"},
        )
        self.assertEqual(
            load1.attributes,
            {
                "jinja2.template_name": "base.html",
                "jinja2.template_path": os.path.join(TMPL_DIR, "base.html"),
            },
        )
        self.assertEqual(
            render.attributes, {"jinja2.template_name": "template.html"},
        )

    def test_uninstrumented(self):
        Jinja2Instrumentor().uninstrument()

        jinja2.environment.Template("Hello {{name}}!")
        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 0)

        Jinja2Instrumentor().instrument()
