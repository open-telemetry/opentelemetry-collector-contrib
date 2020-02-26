from weakref import WeakValueDictionary

from .constants import CTX_KEY


def tags_from_context(context):
    """Helper to extract meta values from a Celery Context"""
    tag_keys = (
        'compression', 'correlation_id', 'countdown', 'delivery_info', 'eta',
        'exchange', 'expires', 'hostname', 'id', 'priority', 'queue', 'reply_to',
        'retries', 'routing_key', 'serializer', 'timelimit', 'origin', 'state',
    )

    tags = {}
    for key in tag_keys:
        value = context.get(key)

        # Skip this key if it is not set
        if value is None or value == '':
            continue

        # Skip `timelimit` if it is not set (it's default/unset value is a
        # tuple or a list of `None` values
        if key == 'timelimit' and value in [(None, None), [None, None]]:
            continue

        # Skip `retries` if it's value is `0`
        if key == 'retries' and value == 0:
            continue

        # Celery 4.0 uses `origin` instead of `hostname`; this change preserves
        # the same name for the tag despite Celery version
        if key == 'origin':
            key = 'hostname'

        # prefix the tag as 'celery'
        tag_name = 'celery.{}'.format(key)
        tags[tag_name] = value
    return tags


def attach_span(task, task_id, span, is_publish=False):
    """Helper to propagate a `Span` for the given `Task` instance. This
    function uses a `WeakValueDictionary` that stores a Datadog Span using
    the `(task_id, is_publish)` as a key. This is useful when information must be
    propagated from one Celery signal to another.

    DEV: We use (task_id, is_publish) for the key to ensure that publishing a
         task from within another task does not cause any conflicts.

         This mostly happens when either a task fails and a retry policy is in place,
         or when a task is manually retries (e.g. `task.retry()`), we end up trying
         to publish a task with the same id as the task currently running.

         Previously publishing the new task would overwrite the existing `celery.run` span
         in the `weak_dict` causing that span to be forgotten and never finished.

         NOTE: We cannot test for this well yet, because we do not run a celery worker,
         and cannot run `task.apply_async()`
    """
    weak_dict = getattr(task, CTX_KEY, None)
    if weak_dict is None:
        weak_dict = WeakValueDictionary()
        setattr(task, CTX_KEY, weak_dict)

    weak_dict[(task_id, is_publish)] = span


def detach_span(task, task_id, is_publish=False):
    """Helper to remove a `Span` in a Celery task when it's propagated.
    This function handles tasks where the `Span` is not attached.
    """
    weak_dict = getattr(task, CTX_KEY, None)
    if weak_dict is None:
        return

    # DEV: See note in `attach_span` for key info
    weak_dict.pop((task_id, is_publish), None)


def retrieve_span(task, task_id, is_publish=False):
    """Helper to retrieve an active `Span` stored in a `Task`
    instance
    """
    weak_dict = getattr(task, CTX_KEY, None)
    if weak_dict is None:
        return
    else:
        # DEV: See note in `attach_span` for key info
        return weak_dict.get((task_id, is_publish))


def retrieve_task_id(context):
    """Helper to retrieve the `Task` identifier from the message `body`.
    This helper supports Protocol Version 1 and 2. The Protocol is well
    detailed in the official documentation:
    http://docs.celeryproject.org/en/latest/internals/protocol.html
    """
    headers = context.get('headers')
    body = context.get('body')
    if headers:
        # Protocol Version 2 (default from Celery 4.0)
        return headers.get('id')
    else:
        # Protocol Version 1
        return body.get('id')
