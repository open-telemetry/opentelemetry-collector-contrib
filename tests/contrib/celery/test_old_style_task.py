import celery

from .base import CeleryBaseTestCase


class CeleryOldStyleTaskTest(CeleryBaseTestCase):
    """Ensure Old Style Tasks are properly instrumented"""

    def test_apply_async_previous_style_tasks(self):
        # ensures apply_async is properly patched if Celery 1.0 style tasks
        # are used even in newer versions. This should extend support to
        # previous versions of Celery.
        # Regression test: https://github.com/DataDog/dd-trace-py/pull/449
        class CelerySuperClass(celery.task.Task):
            abstract = True

            @classmethod
            def apply_async(cls, args=None, kwargs=None, **kwargs_):
                return super(CelerySuperClass, cls).apply_async(args=args, kwargs=kwargs, **kwargs_)

            def run(self, *args, **kwargs):
                if 'stop' in kwargs:
                    # avoid call loop
                    return
                CelerySubClass.apply_async(args=[], kwargs={'stop': True})

        class CelerySubClass(CelerySuperClass):
            pass

        t = CelerySubClass()
        res = t.apply()

        traces = self.tracer.writer.pop_traces()
        assert 1 == len(traces)
        assert 2 == len(traces[0])
        run_span = traces[0][0]
        assert run_span.error == 0
        assert run_span.name == 'celery.run'
        assert run_span.resource == 'tests.contrib.celery.test_old_style_task.CelerySubClass'
        assert run_span.service == 'celery-worker'
        assert run_span.get_tag('celery.id') == res.task_id
        assert run_span.get_tag('celery.action') == 'run'
        assert run_span.get_tag('celery.state') == 'SUCCESS'
        apply_span = traces[0][1]
        assert apply_span.error == 0
        assert apply_span.name == 'celery.apply'
        assert apply_span.resource == 'tests.contrib.celery.test_old_style_task.CelerySubClass'
        assert apply_span.service == 'celery-producer'
        assert apply_span.get_tag('celery.action') == 'apply_async'
        assert apply_span.get_tag('celery.routing_key') == 'celery'
