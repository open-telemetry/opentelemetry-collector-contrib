import celery
from celery.exceptions import Retry

from ddtrace.contrib.celery import patch, unpatch

from .base import CeleryBaseTestCase

from tests.opentracer.utils import init_tracer


class MyException(Exception):
    pass


class CeleryIntegrationTask(CeleryBaseTestCase):
    """Ensures that the tracer works properly with a real Celery application
    without breaking the Application or Task API.
    """

    def test_concurrent_delays(self):
        # it should create one trace for each delayed execution
        @self.app.task
        def fn_task():
            return 42

        for x in range(100):
            fn_task.delay()

        traces = self.tracer.writer.pop_traces()
        assert 100 == len(traces)

    def test_idempotent_patch(self):
        # calling patch() twice doesn't have side effects
        patch()

        @self.app.task
        def fn_task():
            return 42

        t = fn_task.apply()
        assert t.successful()
        assert 42 == t.result

        traces = self.tracer.writer.pop_traces()
        assert 1 == len(traces)
        assert 1 == len(traces[0])

    def test_idempotent_unpatch(self):
        # calling unpatch() twice doesn't have side effects
        unpatch()
        unpatch()

        @self.app.task
        def fn_task():
            return 42

        t = fn_task.apply()
        assert t.successful()
        assert 42 == t.result

        traces = self.tracer.writer.pop_traces()
        assert 0 == len(traces)

    def test_fn_task_run(self):
        # the body of the function is not instrumented so calling it
        # directly doesn't create a trace
        @self.app.task
        def fn_task():
            return 42

        t = fn_task.run()
        assert t == 42

        traces = self.tracer.writer.pop_traces()
        assert 0 == len(traces)

    def test_fn_task_call(self):
        # the body of the function is not instrumented so calling it
        # directly doesn't create a trace
        @self.app.task
        def fn_task():
            return 42

        t = fn_task()
        assert t == 42

        traces = self.tracer.writer.pop_traces()
        assert 0 == len(traces)

    def test_fn_task_apply(self):
        # it should execute a traced task with a returning value
        @self.app.task
        def fn_task():
            return 42

        t = fn_task.apply()
        assert t.successful()
        assert 42 == t.result

        traces = self.tracer.writer.pop_traces()
        assert 1 == len(traces)
        assert 1 == len(traces[0])
        span = traces[0][0]
        assert span.error == 0
        assert span.name == 'celery.run'
        assert span.resource == 'tests.contrib.celery.test_integration.fn_task'
        assert span.service == 'celery-worker'
        assert span.span_type == 'worker'
        assert span.get_tag('celery.id') == t.task_id
        assert span.get_tag('celery.action') == 'run'
        assert span.get_tag('celery.state') == 'SUCCESS'

    def test_fn_task_apply_bind(self):
        # it should execute a traced task with a returning value
        @self.app.task(bind=True)
        def fn_task(self):
            return self

        t = fn_task.apply()
        assert t.successful()
        assert 'fn_task' in t.result.name

        traces = self.tracer.writer.pop_traces()
        assert 1 == len(traces)
        assert 1 == len(traces[0])
        span = traces[0][0]
        assert span.error == 0
        assert span.name == 'celery.run'
        assert span.resource == 'tests.contrib.celery.test_integration.fn_task'
        assert span.service == 'celery-worker'
        assert span.get_tag('celery.id') == t.task_id
        assert span.get_tag('celery.action') == 'run'
        assert span.get_tag('celery.state') == 'SUCCESS'

    def test_fn_task_apply_async(self):
        # it should execute a traced async task that has parameters
        @self.app.task
        def fn_task_parameters(user, force_logout=False):
            return (user, force_logout)

        t = fn_task_parameters.apply_async(args=['user'], kwargs={'force_logout': True})
        assert 'PENDING' == t.status

        traces = self.tracer.writer.pop_traces()
        assert 1 == len(traces)
        assert 1 == len(traces[0])
        span = traces[0][0]
        assert span.error == 0
        assert span.name == 'celery.apply'
        assert span.resource == 'tests.contrib.celery.test_integration.fn_task_parameters'
        assert span.service == 'celery-producer'
        assert span.get_tag('celery.id') == t.task_id
        assert span.get_tag('celery.action') == 'apply_async'
        assert span.get_tag('celery.routing_key') == 'celery'

    def test_fn_task_delay(self):
        # using delay shorthand must preserve arguments
        @self.app.task
        def fn_task_parameters(user, force_logout=False):
            return (user, force_logout)

        t = fn_task_parameters.delay('user', force_logout=True)
        assert 'PENDING' == t.status

        traces = self.tracer.writer.pop_traces()
        assert 1 == len(traces)
        assert 1 == len(traces[0])
        span = traces[0][0]
        assert span.error == 0
        assert span.name == 'celery.apply'
        assert span.resource == 'tests.contrib.celery.test_integration.fn_task_parameters'
        assert span.service == 'celery-producer'
        assert span.get_tag('celery.id') == t.task_id
        assert span.get_tag('celery.action') == 'apply_async'
        assert span.get_tag('celery.routing_key') == 'celery'

    def test_fn_exception(self):
        # it should catch exceptions in task functions
        @self.app.task
        def fn_exception():
            raise Exception('Task class is failing')

        t = fn_exception.apply()
        assert t.failed()
        assert 'Task class is failing' in t.traceback

        traces = self.tracer.writer.pop_traces()
        assert 1 == len(traces)
        assert 1 == len(traces[0])
        span = traces[0][0]
        assert span.name == 'celery.run'
        assert span.resource == 'tests.contrib.celery.test_integration.fn_exception'
        assert span.service == 'celery-worker'
        assert span.get_tag('celery.id') == t.task_id
        assert span.get_tag('celery.action') == 'run'
        assert span.get_tag('celery.state') == 'FAILURE'
        assert span.error == 1
        assert span.get_tag('error.msg') == 'Task class is failing'
        assert 'Traceback (most recent call last)' in span.get_tag('error.stack')
        assert 'Task class is failing' in span.get_tag('error.stack')

    def test_fn_exception_expected(self):
        # it should catch exceptions in task functions
        @self.app.task(throws=(MyException,))
        def fn_exception():
            raise MyException('Task class is failing')

        t = fn_exception.apply()
        assert t.failed()
        assert 'Task class is failing' in t.traceback

        traces = self.tracer.writer.pop_traces()
        assert 1 == len(traces)
        assert 1 == len(traces[0])
        span = traces[0][0]
        assert span.name == 'celery.run'
        assert span.resource == 'tests.contrib.celery.test_integration.fn_exception'
        assert span.service == 'celery-worker'
        assert span.get_tag('celery.id') == t.task_id
        assert span.get_tag('celery.action') == 'run'
        assert span.get_tag('celery.state') == 'FAILURE'
        assert span.error == 0

    def test_fn_retry_exception(self):
        # it should not catch retry exceptions in task functions
        @self.app.task
        def fn_exception():
            raise Retry('Task class is being retried')

        t = fn_exception.apply()
        assert not t.failed()
        assert 'Task class is being retried' in t.traceback

        traces = self.tracer.writer.pop_traces()
        assert 1 == len(traces)
        assert 1 == len(traces[0])
        span = traces[0][0]
        assert span.name == 'celery.run'
        assert span.resource == 'tests.contrib.celery.test_integration.fn_exception'
        assert span.service == 'celery-worker'
        assert span.get_tag('celery.id') == t.task_id
        assert span.get_tag('celery.action') == 'run'
        assert span.get_tag('celery.state') == 'RETRY'
        assert span.get_tag('celery.retry.reason') == 'Task class is being retried'

        # This type of retrying should not be marked as an exception
        assert span.error == 0
        assert not span.get_tag('error.msg')
        assert not span.get_tag('error.stack')

    def test_class_task(self):
        # it should execute class based tasks with a returning value
        class BaseTask(self.app.Task):
            def run(self):
                return 42

        t = BaseTask()
        # register the Task class if it's available (required in Celery 4.0+)
        register_task = getattr(self.app, 'register_task', None)
        if register_task is not None:
            register_task(t)

        r = t.apply()
        assert r.successful()
        assert 42 == r.result

        traces = self.tracer.writer.pop_traces()
        assert 1 == len(traces)
        assert 1 == len(traces[0])
        span = traces[0][0]
        assert span.error == 0
        assert span.name == 'celery.run'
        assert span.resource == 'tests.contrib.celery.test_integration.BaseTask'
        assert span.service == 'celery-worker'
        assert span.get_tag('celery.id') == r.task_id
        assert span.get_tag('celery.action') == 'run'
        assert span.get_tag('celery.state') == 'SUCCESS'

    def test_class_task_exception(self):
        # it should catch exceptions in class based tasks
        class BaseTask(self.app.Task):
            def run(self):
                raise Exception('Task class is failing')

        t = BaseTask()
        # register the Task class if it's available (required in Celery 4.0+)
        register_task = getattr(self.app, 'register_task', None)
        if register_task is not None:
            register_task(t)

        r = t.apply()
        assert r.failed()
        assert 'Task class is failing' in r.traceback

        traces = self.tracer.writer.pop_traces()
        assert 1 == len(traces)
        assert 1 == len(traces[0])
        span = traces[0][0]
        assert span.name == 'celery.run'
        assert span.resource == 'tests.contrib.celery.test_integration.BaseTask'
        assert span.service == 'celery-worker'
        assert span.get_tag('celery.id') == r.task_id
        assert span.get_tag('celery.action') == 'run'
        assert span.get_tag('celery.state') == 'FAILURE'
        assert span.error == 1
        assert span.get_tag('error.msg') == 'Task class is failing'
        assert 'Traceback (most recent call last)' in span.get_tag('error.stack')
        assert 'Task class is failing' in span.get_tag('error.stack')

    def test_class_task_exception_expected(self):
        # it should catch exceptions in class based tasks
        class BaseTask(self.app.Task):
            throws = (MyException,)

            def run(self):
                raise MyException('Task class is failing')

        t = BaseTask()
        # register the Task class if it's available (required in Celery 4.0+)
        register_task = getattr(self.app, 'register_task', None)
        if register_task is not None:
            register_task(t)

        r = t.apply()
        assert r.failed()
        assert 'Task class is failing' in r.traceback

        traces = self.tracer.writer.pop_traces()
        assert 1 == len(traces)
        assert 1 == len(traces[0])
        span = traces[0][0]
        assert span.name == 'celery.run'
        assert span.resource == 'tests.contrib.celery.test_integration.BaseTask'
        assert span.service == 'celery-worker'
        assert span.get_tag('celery.id') == r.task_id
        assert span.get_tag('celery.action') == 'run'
        assert span.get_tag('celery.state') == 'FAILURE'
        assert span.error == 0

    def test_shared_task(self):
        # Ensure Django Shared Task are supported
        @celery.shared_task
        def add(x, y):
            return x + y

        res = add.apply([2, 2])
        assert res.result == 4

        traces = self.tracer.writer.pop_traces()
        assert 1 == len(traces)
        assert 1 == len(traces[0])
        span = traces[0][0]
        assert span.error == 0
        assert span.name == 'celery.run'
        assert span.service == 'celery-worker'
        assert span.resource == 'tests.contrib.celery.test_integration.add'
        assert span.parent_id is None
        assert span.get_tag('celery.id') == res.task_id
        assert span.get_tag('celery.action') == 'run'
        assert span.get_tag('celery.state') == 'SUCCESS'

    def test_worker_service_name(self):
        @self.app.task
        def fn_task():
            return 42

        # Ensure worker service name can be changed via
        # configuration object
        with self.override_config('celery', dict(worker_service_name='worker-notify')):
            t = fn_task.apply()
            self.assertTrue(t.successful())
            self.assertEqual(42, t.result)

            traces = self.tracer.writer.pop_traces()
            self.assertEqual(1, len(traces))
            self.assertEqual(1, len(traces[0]))
            span = traces[0][0]
            self.assertEqual(span.service, 'worker-notify')

    def test_producer_service_name(self):
        @self.app.task
        def fn_task():
            return 42

        # Ensure producer service name can be changed via
        # configuration object
        with self.override_config('celery', dict(producer_service_name='task-queue')):
            t = fn_task.delay()
            self.assertEqual('PENDING', t.status)

            traces = self.tracer.writer.pop_traces()
            self.assertEqual(1, len(traces))
            self.assertEqual(1, len(traces[0]))
            span = traces[0][0]
            self.assertEqual(span.service, 'task-queue')

    def test_fn_task_apply_async_ot(self):
        """OpenTracing version of test_fn_task_apply_async."""
        ot_tracer = init_tracer('celery_svc', self.tracer)

        # it should execute a traced async task that has parameters
        @self.app.task
        def fn_task_parameters(user, force_logout=False):
            return (user, force_logout)

        with ot_tracer.start_active_span('celery_op'):
            t = fn_task_parameters.apply_async(args=['user'], kwargs={'force_logout': True})
            assert 'PENDING' == t.status

        traces = self.tracer.writer.pop_traces()
        assert 1 == len(traces)
        assert 2 == len(traces[0])
        ot_span, dd_span = traces[0]

        # confirm the parenting
        assert ot_span.parent_id is None
        assert dd_span.parent_id == ot_span.span_id

        assert ot_span.name == 'celery_op'
        assert ot_span.service == 'celery_svc'

        assert dd_span.error == 0
        assert dd_span.name == 'celery.apply'
        assert dd_span.resource == 'tests.contrib.celery.test_integration.fn_task_parameters'
        assert dd_span.service == 'celery-producer'
        assert dd_span.get_tag('celery.id') == t.task_id
        assert dd_span.get_tag('celery.action') == 'apply_async'
        assert dd_span.get_tag('celery.routing_key') == 'celery'
