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

import threading
import time

from opentelemetry.instrumentation.celery import CeleryInstrumentor
from opentelemetry.test.test_base import TestBase
from opentelemetry.trace import SpanKind

from .celery_test_tasks import app, task_add


class TestCeleryInstrumentation(TestBase):
    def setUp(self):
        super().setUp()
        self._worker = app.Worker(app=app, pool="solo", concurrency=1)
        self._thread = threading.Thread(target=self._worker.start)
        self._thread.daemon = True
        self._thread.start()

    def tearDown(self):
        super().tearDown()
        self._worker.stop()
        self._thread.join()

    def test_task(self):
        CeleryInstrumentor().instrument()

        result = task_add.delay(1, 2)
        while not result.ready():
            time.sleep(0.05)

        spans = self.sorted_spans(self.memory_exporter.get_finished_spans())
        self.assertEqual(len(spans), 2)

        consumer, producer = spans

        self.assertEqual(consumer.name, "run/tests.celery_test_tasks.task_add")
        self.assertEqual(consumer.kind, SpanKind.CONSUMER)
        self.assert_span_has_attributes(
            consumer,
            {
                "celery.action": "run",
                "celery.state": "SUCCESS",
                "messaging.destination": "celery",
                "celery.task_name": "tests.celery_test_tasks.task_add",
            },
        )

        self.assertEqual(
            producer.name, "apply_async/tests.celery_test_tasks.task_add"
        )
        self.assertEqual(producer.kind, SpanKind.PRODUCER)
        self.assert_span_has_attributes(
            producer,
            {
                "celery.action": "apply_async",
                "celery.task_name": "tests.celery_test_tasks.task_add",
                "messaging.destination_kind": "queue",
                "messaging.destination": "celery",
            },
        )

        self.assertNotEqual(consumer.parent, producer.context)
        self.assertEqual(consumer.parent.span_id, producer.context.span_id)
        self.assertEqual(consumer.context.trace_id, producer.context.trace_id)
