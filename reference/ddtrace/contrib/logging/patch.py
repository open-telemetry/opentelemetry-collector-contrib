import logging

from ddtrace import config

from ...helpers import get_correlation_ids
from ...utils.wrappers import unwrap as _u
from ...vendor.wrapt import wrap_function_wrapper as _w

RECORD_ATTR_TRACE_ID = 'dd.trace_id'
RECORD_ATTR_SPAN_ID = 'dd.span_id'
RECORD_ATTR_VALUE_NULL = 0

config._add('logging', dict(
    tracer=None,  # by default, override here for custom tracer
))


def _w_makeRecord(func, instance, args, kwargs):
    record = func(*args, **kwargs)

    # add correlation identifiers to LogRecord
    trace_id, span_id = get_correlation_ids(tracer=config.logging.tracer)
    if trace_id and span_id:
        setattr(record, RECORD_ATTR_TRACE_ID, trace_id)
        setattr(record, RECORD_ATTR_SPAN_ID, span_id)
    else:
        setattr(record, RECORD_ATTR_TRACE_ID, RECORD_ATTR_VALUE_NULL)
        setattr(record, RECORD_ATTR_SPAN_ID, RECORD_ATTR_VALUE_NULL)

    return record


def patch():
    """
    Patch ``logging`` module in the Python Standard Library for injection of
    tracer information by wrapping the base factory method ``Logger.makeRecord``
    """
    if getattr(logging, '_datadog_patch', False):
        return
    setattr(logging, '_datadog_patch', True)

    _w(logging.Logger, 'makeRecord', _w_makeRecord)


def unpatch():
    if getattr(logging, '_datadog_patch', False):
        setattr(logging, '_datadog_patch', False)

        _u(logging.Logger, 'makeRecord')
