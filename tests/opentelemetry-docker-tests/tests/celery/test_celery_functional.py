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


import celery
import pytest
from celery.exceptions import Retry

import opentelemetry.instrumentation.celery
from opentelemetry import trace as trace_api
from opentelemetry.instrumentation.celery import CeleryInstrumentor
from opentelemetry.sdk import resources
from opentelemetry.sdk.trace import TracerProvider, export
from opentelemetry.trace.status import StatusCanonicalCode

# set a high timeout for async executions due to issues in CI
ASYNC_GET_TIMEOUT = 120


class MyException(Exception):
    pass


@pytest.mark.skip(reason="inconsistent test results")
def test_instrumentation_info(celery_app, memory_exporter):
    @celery_app.task
    def fn_task():
        return 42

    result = fn_task.apply_async()
    assert result.get(timeout=ASYNC_GET_TIMEOUT) == 42

    spans = memory_exporter.get_finished_spans()
    assert len(spans) == 2

    async_span, run_span = spans

    assert run_span.parent == async_span.context
    assert run_span.parent.span_id == async_span.context.span_id
    assert run_span.context.trace_id == async_span.context.trace_id

    assert async_span.instrumentation_info.name == "apply_async/{0}".format(
        opentelemetry.instrumentation.celery.__name__
    )
    assert async_span.instrumentation_info.version == "apply_async/{0}".format(
        opentelemetry.instrumentation.celery.__version__
    )
    assert run_span.instrumentation_info.name == "run/{0}".format(
        opentelemetry.instrumentation.celery.__name__
    )
    assert run_span.instrumentation_info.version == "run/{0}".format(
        opentelemetry.instrumentation.celery.__version__
    )


def test_fn_task_run(celery_app, memory_exporter):
    @celery_app.task
    def fn_task():
        return 42

    t = fn_task.run()
    assert t == 42

    spans = memory_exporter.get_finished_spans()
    assert len(spans) == 0


def test_fn_task_call(celery_app, memory_exporter):
    @celery_app.task
    def fn_task():
        return 42

    t = fn_task()
    assert t == 42

    spans = memory_exporter.get_finished_spans()
    assert len(spans) == 0


def test_fn_task_apply(celery_app, memory_exporter):
    @celery_app.task
    def fn_task():
        return 42

    t = fn_task.apply()
    assert t.successful() is True
    assert t.result == 42

    spans = memory_exporter.get_finished_spans()
    assert len(spans) == 1

    span = spans[0]

    assert span.status.is_ok is True
    assert span.name == "run/test_celery_functional.fn_task"
    assert span.attributes.get("messaging.message_id") == t.task_id
    assert (
        span.attributes.get("celery.task_name")
        == "test_celery_functional.fn_task"
    )
    assert span.attributes.get("celery.action") == "run"
    assert span.attributes.get("celery.state") == "SUCCESS"


def test_fn_task_apply_bind(celery_app, memory_exporter):
    @celery_app.task(bind=True)
    def fn_task(self):
        return self

    t = fn_task.apply()
    assert t.successful() is True
    assert "fn_task" in t.result.name

    spans = memory_exporter.get_finished_spans()
    assert len(spans) == 1

    span = spans[0]

    assert span.status.is_ok is True
    assert span.name == "run/test_celery_functional.fn_task"
    assert span.attributes.get("messaging.message_id") == t.task_id
    assert (
        span.attributes.get("celery.task_name")
        == "test_celery_functional.fn_task"
    )
    assert span.attributes.get("celery.action") == "run"
    assert span.attributes.get("celery.state") == "SUCCESS"


@pytest.mark.skip(reason="inconsistent test results")
def test_fn_task_apply_async(celery_app, memory_exporter):
    @celery_app.task
    def fn_task_parameters(user, force_logout=False):
        return (user, force_logout)

    result = fn_task_parameters.apply_async(
        args=["user"], kwargs={"force_logout": True}
    )
    assert result.get(timeout=ASYNC_GET_TIMEOUT) == ["user", True]

    spans = memory_exporter.get_finished_spans()
    assert len(spans) == 2

    async_span, run_span = spans

    assert run_span.context.trace_id != async_span.context.trace_id

    assert async_span.status.is_ok is True
    assert (
        async_span.name
        == "apply_async/test_celery_functional.fn_task_parameters"
    )
    assert async_span.attributes.get("celery.action") == "apply_async"
    assert async_span.attributes.get("messaging.message_id") == result.task_id
    assert (
        async_span.attributes.get("celery.task_name")
        == "test_celery_functional.fn_task_parameters"
    )

    assert run_span.status.is_ok is True
    assert run_span.name == "test_celery_functional.fn_task_parameters"
    assert run_span.attributes.get("celery.action") == "run"
    assert run_span.attributes.get("celery.state") == "SUCCESS"
    assert run_span.attributes.get("messaging.message_id") == result.task_id
    assert (
        run_span.attributes.get("celery.task_name")
        == "test_celery_functional.fn_task_parameters"
    )


@pytest.mark.skip(reason="inconsistent test results")
def test_concurrent_delays(celery_app, memory_exporter):
    @celery_app.task
    def fn_task():
        return 42

    results = [fn_task.delay() for _ in range(100)]

    for result in results:
        assert result.get(timeout=ASYNC_GET_TIMEOUT) == 42

    spans = memory_exporter.get_finished_spans()

    assert len(spans) == 200


@pytest.mark.skip(reason="inconsistent test results")
def test_fn_task_delay(celery_app, memory_exporter):
    @celery_app.task
    def fn_task_parameters(user, force_logout=False):
        return (user, force_logout)

    result = fn_task_parameters.delay("user", force_logout=True)
    assert result.get(timeout=ASYNC_GET_TIMEOUT) == ["user", True]

    spans = memory_exporter.get_finished_spans()
    assert len(spans) == 2

    async_span, run_span = spans

    assert run_span.context.trace_id != async_span.context.trace_id

    assert async_span.status.is_ok is True
    assert (
        async_span.name
        == "apply_async/test_celery_functional.fn_task_parameters"
    )
    assert async_span.attributes.get("celery.action") == "apply_async"
    assert async_span.attributes.get("messaging.message_id") == result.task_id
    assert (
        async_span.attributes.get("celery.task_name")
        == "test_celery_functional.fn_task_parameters"
    )

    assert run_span.status.is_ok is True
    assert run_span.name == "run/test_celery_functional.fn_task_parameters"
    assert run_span.attributes.get("celery.action") == "run"
    assert run_span.attributes.get("celery.state") == "SUCCESS"
    assert run_span.attributes.get("messaging.message_id") == result.task_id
    assert (
        run_span.attributes.get("celery.task_name")
        == "test_celery_functional.fn_task_parameters"
    )


def test_fn_exception(celery_app, memory_exporter):
    @celery_app.task
    def fn_exception():
        raise Exception("Task class is failing")

    result = fn_exception.apply()

    assert result.failed() is True
    assert "Task class is failing" in result.traceback

    spans = memory_exporter.get_finished_spans()
    assert len(spans) == 1

    span = spans[0]

    assert span.status.is_ok is False
    assert span.name == "run/test_celery_functional.fn_exception"
    assert span.attributes.get("celery.action") == "run"
    assert span.attributes.get("celery.state") == "FAILURE"
    assert (
        span.attributes.get("celery.task_name")
        == "test_celery_functional.fn_exception"
    )
    assert span.status.canonical_code == StatusCanonicalCode.UNKNOWN
    assert span.attributes.get("messaging.message_id") == result.task_id
    assert "Task class is failing" in span.status.description


def test_fn_exception_expected(celery_app, memory_exporter):
    @celery_app.task(throws=(MyException,))
    def fn_exception():
        raise MyException("Task class is failing")

    result = fn_exception.apply()

    assert result.failed() is True
    assert "Task class is failing" in result.traceback

    spans = memory_exporter.get_finished_spans()
    assert len(spans) == 1

    span = spans[0]

    assert span.status.is_ok is True
    assert span.status.canonical_code == StatusCanonicalCode.OK
    assert span.name == "run/test_celery_functional.fn_exception"
    assert span.attributes.get("celery.action") == "run"
    assert span.attributes.get("celery.state") == "FAILURE"
    assert (
        span.attributes.get("celery.task_name")
        == "test_celery_functional.fn_exception"
    )
    assert span.attributes.get("messaging.message_id") == result.task_id


def test_fn_retry_exception(celery_app, memory_exporter):
    @celery_app.task
    def fn_exception():
        raise Retry("Task class is being retried")

    result = fn_exception.apply()

    assert result.failed() is False
    assert "Task class is being retried" in result.traceback

    spans = memory_exporter.get_finished_spans()
    assert len(spans) == 1

    span = spans[0]

    assert span.status.is_ok is True
    assert span.status.canonical_code == StatusCanonicalCode.OK
    assert span.name == "run/test_celery_functional.fn_exception"
    assert span.attributes.get("celery.action") == "run"
    assert span.attributes.get("celery.state") == "RETRY"
    assert (
        span.attributes.get("celery.task_name")
        == "test_celery_functional.fn_exception"
    )
    assert span.attributes.get("messaging.message_id") == result.task_id


def test_class_task(celery_app, memory_exporter):
    class BaseTask(celery_app.Task):
        def run(self):
            return 42

    task = BaseTask()
    # register the Task class if it's available (required in Celery 4.0+)
    register_task = getattr(celery_app, "register_task", None)
    if register_task is not None:
        register_task(task)

    result = task.apply()

    assert result.successful() is True
    assert result.result == 42

    spans = memory_exporter.get_finished_spans()
    assert len(spans) == 1

    span = spans[0]

    assert span.status.is_ok is True
    assert span.name == "run/test_celery_functional.BaseTask"
    assert (
        span.attributes.get("celery.task_name")
        == "test_celery_functional.BaseTask"
    )
    assert span.attributes.get("celery.action") == "run"
    assert span.attributes.get("celery.state") == "SUCCESS"
    assert span.attributes.get("messaging.message_id") == result.task_id


def test_class_task_exception(celery_app, memory_exporter):
    class BaseTask(celery_app.Task):
        def run(self):
            raise Exception("Task class is failing")

    task = BaseTask()
    # register the Task class if it's available (required in Celery 4.0+)
    register_task = getattr(celery_app, "register_task", None)
    if register_task is not None:
        register_task(task)

    result = task.apply()

    assert result.failed() is True
    assert "Task class is failing" in result.traceback

    spans = memory_exporter.get_finished_spans()
    assert len(spans) == 1

    span = spans[0]

    assert span.status.is_ok is False
    assert span.name == "run/test_celery_functional.BaseTask"
    assert (
        span.attributes.get("celery.task_name")
        == "test_celery_functional.BaseTask"
    )
    assert span.attributes.get("celery.action") == "run"
    assert span.attributes.get("celery.state") == "FAILURE"
    assert span.status.canonical_code == StatusCanonicalCode.UNKNOWN
    assert span.attributes.get("messaging.message_id") == result.task_id
    assert "Task class is failing" in span.status.description


def test_class_task_exception_excepted(celery_app, memory_exporter):
    class BaseTask(celery_app.Task):
        throws = (MyException,)

        def run(self):
            raise MyException("Task class is failing")

    task = BaseTask()
    # register the Task class if it's available (required in Celery 4.0+)
    register_task = getattr(celery_app, "register_task", None)
    if register_task is not None:
        register_task(task)

    result = task.apply()

    assert result.failed() is True
    assert "Task class is failing" in result.traceback

    spans = memory_exporter.get_finished_spans()
    assert len(spans) == 1

    span = spans[0]

    assert span.status.is_ok is True
    assert span.status.canonical_code == StatusCanonicalCode.OK
    assert span.name == "run/test_celery_functional.BaseTask"
    assert span.attributes.get("celery.action") == "run"
    assert span.attributes.get("celery.state") == "FAILURE"
    assert span.attributes.get("messaging.message_id") == result.task_id


def test_shared_task(celery_app, memory_exporter):
    """Ensure Django Shared Task are supported"""

    @celery.shared_task
    def add(x, y):
        return x + y

    result = add.apply([2, 2])
    assert result.result == 4

    spans = memory_exporter.get_finished_spans()
    assert len(spans) == 1

    span = spans[0]

    assert span.status.is_ok is True
    assert span.name == "run/test_celery_functional.add"
    assert (
        span.attributes.get("celery.task_name") == "test_celery_functional.add"
    )
    assert span.attributes.get("celery.action") == "run"
    assert span.attributes.get("celery.state") == "SUCCESS"
    assert span.attributes.get("messaging.message_id") == result.task_id


@pytest.mark.skip(reason="inconsistent test results")
def test_apply_async_previous_style_tasks(
    celery_app, celery_worker, memory_exporter
):
    """Ensures apply_async is properly patched if Celery 1.0 style tasks are
    used even in newer versions. This should extend support to previous versions
    of Celery."""

    class CelerySuperClass(celery.task.Task):
        abstract = True

        @classmethod
        def apply_async(cls, args=None, kwargs=None, **kwargs_):
            return super(CelerySuperClass, cls).apply_async(
                args=args, kwargs=kwargs, **kwargs_
            )

        def run(self, *args, **kwargs):
            if "stop" in kwargs:
                # avoid call loop
                return
            CelerySubClass.apply_async(args=[], kwargs={"stop": True}).get(
                timeout=ASYNC_GET_TIMEOUT
            )

    class CelerySubClass(CelerySuperClass):
        pass

    celery_worker.reload()

    task = CelerySubClass()
    result = task.apply()

    spans = memory_exporter.get_finished_spans()
    assert len(spans) == 3

    async_span, async_run_span, run_span = spans

    assert run_span.status.is_ok is True
    assert run_span.name == "run/test_celery_functional.CelerySubClass"
    assert (
        run_span.attributes.get("celery.task_name")
        == "test_celery_functional.CelerySubClass"
    )
    assert run_span.attributes.get("celery.action") == "run"
    assert run_span.attributes.get("celery.state") == "SUCCESS"
    assert run_span.attributes.get("messaging.message_id") == result.task_id

    assert async_run_span.status.is_ok is True
    assert async_run_span.name == "run/test_celery_functional.CelerySubClass"
    assert (
        async_run_span.attributes.get("celery.task_name")
        == "test_celery_functional.CelerySubClass"
    )
    assert async_run_span.attributes.get("celery.action") == "run"
    assert async_run_span.attributes.get("celery.state") == "SUCCESS"
    assert (
        async_run_span.attributes.get("messaging.message_id") != result.task_id
    )

    assert async_span.status.is_ok is True
    assert (
        async_span.name == "apply_async/test_celery_functional.CelerySubClass"
    )
    assert (
        async_span.attributes.get("celery.task_name")
        == "test_celery_functional.CelerySubClass"
    )
    assert async_span.attributes.get("celery.action") == "apply_async"
    assert async_span.attributes.get("messaging.message_id") != result.task_id
    assert async_span.attributes.get(
        "messaging.message_id"
    ) == async_run_span.attributes.get("messaging.message_id")


def test_custom_tracer_provider(celery_app, memory_exporter):
    @celery_app.task
    def fn_task():
        return 42

    resource = resources.Resource.create({})
    tracer_provider = TracerProvider(resource=resource)
    span_processor = export.SimpleExportSpanProcessor(memory_exporter)
    tracer_provider.add_span_processor(span_processor)

    trace_api.set_tracer_provider(tracer_provider)

    CeleryInstrumentor().uninstrument()
    CeleryInstrumentor().instrument(tracer_provider=tracer_provider)

    fn_task.delay()

    spans_list = memory_exporter.get_finished_spans()
    assert len(spans_list) == 1

    span = spans_list[0]
    assert span.resource == resource
