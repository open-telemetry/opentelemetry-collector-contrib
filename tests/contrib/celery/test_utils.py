import gc

from ddtrace.contrib.celery.utils import (
    tags_from_context,
    retrieve_task_id,
    attach_span,
    detach_span,
    retrieve_span,
)

from .base import CeleryBaseTestCase


class CeleryTagsTest(CeleryBaseTestCase):
    """Ensures that Celery doesn't extract too much meta
    data when executing tasks asynchronously.
    """
    def test_tags_from_context(self):
        # it should extract only relevant keys
        context = {
            'correlation_id': '44b7f305',
            'delivery_info': '{"eager": "True"}',
            'eta': 'soon',
            'expires': 'later',
            'hostname': 'localhost',
            'id': '44b7f305',
            'reply_to': '44b7f305',
            'retries': 4,
            'timelimit': ('now', 'later'),
            'custom_meta': 'custom_value',
        }

        metas = tags_from_context(context)
        assert metas['celery.correlation_id'] == '44b7f305'
        assert metas['celery.delivery_info'] == '{"eager": "True"}'
        assert metas['celery.eta'] == 'soon'
        assert metas['celery.expires'] == 'later'
        assert metas['celery.hostname'] == 'localhost'
        assert metas['celery.id'] == '44b7f305'
        assert metas['celery.reply_to'] == '44b7f305'
        assert metas['celery.retries'] == 4
        assert metas['celery.timelimit'] == ('now', 'later')
        assert metas.get('custom_meta', None) is None

    def test_tags_from_context_empty_keys(self):
        # it should not extract empty keys
        context = {
            'correlation_id': None,
            'exchange': '',
            'timelimit': (None, None),
            'retries': 0,
        }

        tags = tags_from_context(context)
        assert {} == tags
        # edge case: `timelimit` can also be a list of None values
        context = {
            'timelimit': [None, None],
        }

        tags = tags_from_context(context)
        assert {} == tags

    def test_span_propagation(self):
        # ensure spans getter and setter works properly
        @self.app.task
        def fn_task():
            return 42

        # propagate and retrieve a Span
        task_id = '7c6731af-9533-40c3-83a9-25b58f0d837f'
        span_before = self.tracer.trace('celery.run')
        attach_span(fn_task, task_id, span_before)
        span_after = retrieve_span(fn_task, task_id)
        assert span_before is span_after

    def test_span_delete(self):
        # ensure the helper removes properly a propagated Span
        @self.app.task
        def fn_task():
            return 42

        # propagate a Span
        task_id = '7c6731af-9533-40c3-83a9-25b58f0d837f'
        span = self.tracer.trace('celery.run')
        attach_span(fn_task, task_id, span)
        # delete the Span
        weak_dict = getattr(fn_task, '__dd_task_span')
        detach_span(fn_task, task_id)
        assert weak_dict.get((task_id, False)) is None

    def test_span_delete_empty(self):
        # ensure the helper works even if the Task doesn't have
        # a propagation
        @self.app.task
        def fn_task():
            return 42

        # delete the Span
        exception = None
        task_id = '7c6731af-9533-40c3-83a9-25b58f0d837f'
        try:
            detach_span(fn_task, task_id)
        except Exception as e:
            exception = e
        assert exception is None

    def test_memory_leak_safety(self):
        # Spans are shared between signals using a Dictionary (task_id -> span).
        # This test ensures the GC correctly cleans finished spans. If this test
        # fails a memory leak will happen for sure.
        @self.app.task
        def fn_task():
            return 42

        # propagate and finish a Span for `fn_task`
        task_id = '7c6731af-9533-40c3-83a9-25b58f0d837f'
        attach_span(fn_task, task_id, self.tracer.trace('celery.run'))
        weak_dict = getattr(fn_task, '__dd_task_span')
        key = (task_id, False)
        assert weak_dict.get(key)
        # flush data and force the GC
        weak_dict.get(key).finish()
        self.tracer.writer.pop()
        self.tracer.writer.pop_traces()
        gc.collect()
        assert weak_dict.get(key) is None

    def test_task_id_from_protocol_v1(self):
        # ensures a `task_id` is properly returned when Protocol v1 is used.
        # `context` is an example of an emitted Signal with Protocol v1
        context = {
            'body': {
                'expires': None,
                'utc': True,
                'args': ['user'],
                'chord': None,
                'callbacks': None,
                'errbacks': None,
                'taskset': None,
                'id': 'dffcaec1-dd92-4a1a-b3ab-d6512f4beeb7',
                'retries': 0,
                'task': 'tests.contrib.celery.test_integration.fn_task_parameters',
                'timelimit': (None, None),
                'eta': None,
                'kwargs': {'force_logout': True}
            },
            'sender': 'tests.contrib.celery.test_integration.fn_task_parameters',
            'exchange': 'celery',
            'routing_key': 'celery',
            'retry_policy': None,
            'headers': {},
            'properties': {},
        }

        task_id = retrieve_task_id(context)
        assert task_id == 'dffcaec1-dd92-4a1a-b3ab-d6512f4beeb7'

    def test_task_id_from_protocol_v2(self):
        # ensures a `task_id` is properly returned when Protocol v2 is used.
        # `context` is an example of an emitted Signal with Protocol v2
        context = {
            'body': (
                ['user'],
                {'force_logout': True},
                {u'chord': None, u'callbacks': None, u'errbacks': None, u'chain': None},
            ),
            'sender': u'tests.contrib.celery.test_integration.fn_task_parameters',
            'exchange': u'',
            'routing_key': u'celery',
            'retry_policy': None,
            'headers': {
                u'origin': u'gen83744@hostname',
                u'root_id': '7e917b83-4018-431d-9832-73a28e1fb6c0',
                u'expires': None,
                u'shadow': None,
                u'id': '7e917b83-4018-431d-9832-73a28e1fb6c0',
                u'kwargsrepr': u"{'force_logout': True}",
                u'lang': u'py',
                u'retries': 0,
                u'task': u'tests.contrib.celery.test_integration.fn_task_parameters',
                u'group': None,
                u'timelimit': [None, None],
                u'parent_id': None,
                u'argsrepr': u"['user']",
                u'eta': None,
            },
            'properties': {
                u'reply_to': 'c3054a07-5b28-3855-b18c-1623a24aaeca',
                u'correlation_id': '7e917b83-4018-431d-9832-73a28e1fb6c0',
            },
        }

        task_id = retrieve_task_id(context)
        assert task_id == '7e917b83-4018-431d-9832-73a28e1fb6c0'
