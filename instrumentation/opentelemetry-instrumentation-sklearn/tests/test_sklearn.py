# Copyright 2020, OpenTelemetry Authors
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

from sklearn.ensemble import RandomForestClassifier

from opentelemetry.instrumentation.sklearn import (
    DEFAULT_EXCLUDE_CLASSES,
    DEFAULT_METHODS,
    SklearnInstrumentor,
    get_base_estimators,
    get_delegator,
)
from opentelemetry.test.test_base import TestBase
from opentelemetry.trace import SpanKind

from .fixtures import pipeline, random_input


def assert_instrumented(base_estimators):
    for _, estimator in base_estimators.items():
        for method_name in DEFAULT_METHODS:
            original_method_name = "_otel_original_" + method_name
            if issubclass(estimator, tuple(DEFAULT_EXCLUDE_CLASSES)):
                assert not hasattr(estimator, original_method_name)
                continue
            class_attr = getattr(estimator, method_name, None)
            if isinstance(class_attr, property):
                assert not hasattr(estimator, original_method_name)
                continue
            delegator = None
            if hasattr(estimator, method_name):
                delegator = get_delegator(estimator, method_name)
            if delegator is not None:
                assert hasattr(delegator, "_otel_original_fn")
            elif hasattr(estimator, method_name):
                assert hasattr(estimator, original_method_name)


def assert_uninstrumented(base_estimators):
    for _, estimator in base_estimators.items():
        for method_name in DEFAULT_METHODS:
            original_method_name = "_otel_original_" + method_name
            if issubclass(estimator, tuple(DEFAULT_EXCLUDE_CLASSES)):
                assert not hasattr(estimator, original_method_name)
                continue
            class_attr = getattr(estimator, method_name, None)
            if isinstance(class_attr, property):
                assert not hasattr(estimator, original_method_name)
                continue
            delegator = None
            if hasattr(estimator, method_name):
                delegator = get_delegator(estimator, method_name)
            if delegator is not None:
                assert not hasattr(delegator, "_otel_original_fn")
            elif hasattr(estimator, method_name):
                assert not hasattr(estimator, original_method_name)


class TestSklearn(TestBase):
    def test_package_instrumentation(self):
        ski = SklearnInstrumentor()

        base_estimators = get_base_estimators(packages=["sklearn"])

        model = pipeline()

        ski.instrument()
        assert_instrumented(base_estimators)

        x_test = random_input()

        model.predict(x_test)

        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 8)
        self.memory_exporter.clear()

        ski.uninstrument()
        assert_uninstrumented(base_estimators)

        model = pipeline()
        x_test = random_input()

        model.predict(x_test)

        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 0)

    def test_span_properties(self):
        """Test that we get all of the spans we expect."""
        model = pipeline()
        ski = SklearnInstrumentor()
        ski.instrument_estimator(estimator=model)

        x_test = random_input()

        model.predict(x_test)

        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 8)
        span = spans[0]
        self.assertEqual(span.name, "StandardScaler.transform")
        self.assertEqual(span.kind, SpanKind.INTERNAL)
        self.assertEqual(span.parent.span_id, spans[-1].context.span_id)
        span = spans[1]
        self.assertEqual(span.name, "Normalizer.transform")
        self.assertEqual(span.kind, SpanKind.INTERNAL)
        self.assertEqual(span.parent.span_id, spans[-1].context.span_id)
        span = spans[2]
        self.assertEqual(span.name, "PCA.transform")
        self.assertEqual(span.kind, SpanKind.INTERNAL)
        self.assertEqual(span.parent.span_id, spans[4].context.span_id)
        span = spans[3]
        self.assertEqual(span.name, "TruncatedSVD.transform")
        self.assertEqual(span.kind, SpanKind.INTERNAL)
        self.assertEqual(span.parent.span_id, spans[4].context.span_id)
        span = spans[4]
        self.assertEqual(span.name, "FeatureUnion.transform")
        self.assertEqual(span.kind, SpanKind.INTERNAL)
        self.assertEqual(span.parent.span_id, spans[-1].context.span_id)
        span = spans[5]
        self.assertEqual(span.name, "RandomForestClassifier.predict_proba")
        self.assertEqual(span.kind, SpanKind.INTERNAL)
        self.assertEqual(span.parent.span_id, spans[6].context.span_id)
        span = spans[6]
        self.assertEqual(span.name, "RandomForestClassifier.predict")
        self.assertEqual(span.kind, SpanKind.INTERNAL)
        self.assertEqual(span.parent.span_id, spans[-1].context.span_id)
        span = spans[7]
        self.assertEqual(span.name, "Pipeline.predict")
        self.assertEqual(span.kind, SpanKind.INTERNAL)

        self.memory_exporter.clear()

        # uninstrument
        ski.uninstrument_estimator(estimator=model)
        x_test = random_input()
        model.predict(x_test)
        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 0)

    def test_attrib_config(self):
        """Test that the attribute config makes spans on the decision trees."""
        model = pipeline()
        attrib_config = {RandomForestClassifier: ["estimators_"]}
        ski = SklearnInstrumentor(
            recurse_attribs=attrib_config,
            exclude_classes=[],  # decision trees excluded by default
        )
        ski.instrument_estimator(estimator=model)

        x_test = random_input()
        model.predict(x_test)

        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 8 + model.steps[-1][-1].n_estimators)

        self.memory_exporter.clear()

        ski.uninstrument_estimator(estimator=model)
        x_test = random_input()
        model.predict(x_test)
        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 0)

    def test_span_attributes(self):
        model = pipeline()
        attributes = {"model_name": "random_forest_model"}
        ski = SklearnInstrumentor()
        ski.instrument_estimator(estimator=model, attributes=attributes)

        x_test = random_input()

        model.predict(x_test)

        spans = self.memory_exporter.get_finished_spans()
        for span in spans:
            assert span.attributes["model_name"] == "random_forest_model"
