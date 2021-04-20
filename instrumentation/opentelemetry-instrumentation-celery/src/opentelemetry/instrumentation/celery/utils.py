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

import logging

from celery import registry  # pylint: disable=no-name-in-module

from opentelemetry.semconv.trace import SpanAttributes

logger = logging.getLogger(__name__)

# Celery Context key
CTX_KEY = "__otel_task_span"

# Celery Context attributes
CELERY_CONTEXT_ATTRIBUTES = (
    "compression",
    "correlation_id",
    "countdown",
    "delivery_info",
    "declare",
    "eta",
    "exchange",
    "expires",
    "hostname",
    "id",
    "priority",
    "queue",
    "reply_to",
    "retries",
    "routing_key",
    "serializer",
    "timelimit",
    "origin",
    "state",
)


# pylint:disable=too-many-branches
def set_attributes_from_context(span, context):
    """Helper to extract meta values from a Celery Context"""
    if not span.is_recording():
        return
    for key in CELERY_CONTEXT_ATTRIBUTES:
        value = context.get(key)

        # Skip this key if it is not set
        if value is None or value == "":
            continue

        # Skip `timelimit` if it is not set (it's default/unset value is a
        # tuple or a list of `None` values
        if key == "timelimit" and value in [(None, None), [None, None]]:
            continue

        # Skip `retries` if it's value is `0`
        if key == "retries" and value == 0:
            continue

        attribute_name = None

        # Celery 4.0 uses `origin` instead of `hostname`; this change preserves
        # the same name for the tag despite Celery version
        if key == "origin":
            key = "hostname"

        elif key == "delivery_info":
            # Get also destination from this
            routing_key = value.get("routing_key")
            if routing_key is not None:
                span.set_attribute(
                    SpanAttributes.MESSAGING_DESTINATION, routing_key
                )
            value = str(value)

        elif key == "id":
            attribute_name = SpanAttributes.MESSAGING_MESSAGE_ID

        elif key == "correlation_id":
            attribute_name = SpanAttributes.MESSAGING_CONVERSATION_ID

        elif key == "routing_key":
            attribute_name = SpanAttributes.MESSAGING_DESTINATION

        # according to https://docs.celeryproject.org/en/stable/userguide/routing.html#exchange-types
        elif key == "declare":
            attribute_name = SpanAttributes.MESSAGING_DESTINATION_KIND
            for declare in value:
                if declare.exchange.type == "direct":
                    value = "queue"
                    break
                if declare.exchange.type == "topic":
                    value = "topic"
                    break

        # set attribute name if not set specially for a key
        if attribute_name is None:
            attribute_name = "celery.{}".format(key)

        span.set_attribute(attribute_name, value)


def attach_span(task, task_id, span, is_publish=False):
    """Helper to propagate a `Span` for the given `Task` instance. This
    function uses a `dict` that stores the Span using the
    `(task_id, is_publish)` as a key. This is useful when information must be
    propagated from one Celery signal to another.

    We use (task_id, is_publish) for the key to ensure that publishing a
    task from within another task does not cause any conflicts.

    This mostly happens when either a task fails and a retry policy is in place,
    or when a task is manually retries (e.g. `task.retry()`), we end up trying
    to publish a task with the same id as the task currently running.

    Previously publishing the new task would overwrite the existing `celery.run` span
    in the `dict` causing that span to be forgotten and never finished
    NOTE: We cannot test for this well yet, because we do not run a celery worker,
    and cannot run `task.apply_async()`
    """
    span_dict = getattr(task, CTX_KEY, None)
    if span_dict is None:
        span_dict = dict()
        setattr(task, CTX_KEY, span_dict)

    span_dict[(task_id, is_publish)] = span


def detach_span(task, task_id, is_publish=False):
    """Helper to remove a `Span` in a Celery task when it's propagated.
    This function handles tasks where the `Span` is not attached.
    """
    span_dict = getattr(task, CTX_KEY, None)
    if span_dict is None:
        return

    # See note in `attach_span` for key info
    span_dict.pop((task_id, is_publish), (None, None))


def retrieve_span(task, task_id, is_publish=False):
    """Helper to retrieve an active `Span` stored in a `Task`
    instance
    """
    span_dict = getattr(task, CTX_KEY, None)
    if span_dict is None:
        return (None, None)

    # See note in `attach_span` for key info
    return span_dict.get((task_id, is_publish), (None, None))


def retrieve_task(kwargs):
    task = kwargs.get("task")
    if task is None:
        logger.debug("Unable to retrieve task from signal arguments")
    return task


def retrieve_task_from_sender(kwargs):
    sender = kwargs.get("sender")
    if sender is None:
        logger.debug("Unable to retrieve the sender from signal arguments")

    # before and after publish signals sender is the task name
    # for retry and failure signals sender is the task object
    if isinstance(sender, str):
        sender = registry.tasks.get(sender)
        if sender is None:
            logger.debug("Unable to retrieve the task from sender=%s", sender)

    return sender


def retrieve_task_id(kwargs):
    task_id = kwargs.get("task_id")
    if task_id is None:
        logger.debug("Unable to retrieve task_id from signal arguments")
    return task_id


def retrieve_task_id_from_request(kwargs):
    # retry signal does not include task_id as argument so use request argument
    request = kwargs.get("request")
    if request is None:
        logger.debug("Unable to retrieve the request from signal arguments")

    task_id = getattr(request, "id")
    if task_id is None:
        logger.debug("Unable to retrieve the task_id from the request")

    return task_id


def retrieve_task_id_from_message(kwargs):
    """Helper to retrieve the `Task` identifier from the message `body`.
    This helper supports Protocol Version 1 and 2. The Protocol is well
    detailed in the official documentation:
    http://docs.celeryproject.org/en/latest/internals/protocol.html
    """
    headers = kwargs.get("headers")
    body = kwargs.get("body")
    if headers is not None and len(headers) > 0:
        # Protocol Version 2 (default from Celery 4.0)
        return headers.get("id")
    # Protocol Version 1
    return body.get("id")


def retrieve_reason(kwargs):
    reason = kwargs.get("reason")
    if not reason:
        logger.debug("Unable to retrieve the retry reason")
    return reason
