from importlib import import_module

from ddtrace.vendor.wrapt import wrap_function_wrapper as _w

from .quantize import quantize

from ...compat import urlencode
from ...constants import ANALYTICS_SAMPLE_RATE_KEY
from ...ext import SpanTypes, elasticsearch as metadata, http
from ...pin import Pin
from ...utils.wrappers import unwrap as _u
from ...settings import config


def _es_modules():
    module_names = ('elasticsearch', 'elasticsearch1', 'elasticsearch2', 'elasticsearch5', 'elasticsearch6')
    for module_name in module_names:
        try:
            yield import_module(module_name)
        except ImportError:
            pass


# NB: We are patching the default elasticsearch.transport module
def patch():
    for elasticsearch in _es_modules():
        _patch(elasticsearch)


def _patch(elasticsearch):
    if getattr(elasticsearch, '_datadog_patch', False):
        return
    setattr(elasticsearch, '_datadog_patch', True)
    _w(elasticsearch.transport, 'Transport.perform_request', _get_perform_request(elasticsearch))
    Pin(service=metadata.SERVICE, app=metadata.APP).onto(elasticsearch.transport.Transport)


def unpatch():
    for elasticsearch in _es_modules():
        _unpatch(elasticsearch)


def _unpatch(elasticsearch):
    if getattr(elasticsearch, '_datadog_patch', False):
        setattr(elasticsearch, '_datadog_patch', False)
        _u(elasticsearch.transport.Transport, 'perform_request')


def _get_perform_request(elasticsearch):
    def _perform_request(func, instance, args, kwargs):
        pin = Pin.get_from(instance)
        if not pin or not pin.enabled():
            return func(*args, **kwargs)

        with pin.tracer.trace('elasticsearch.query', span_type=SpanTypes.ELASTICSEARCH) as span:
            # Don't instrument if the trace is not sampled
            if not span.sampled:
                return func(*args, **kwargs)

            method, url = args
            params = kwargs.get('params')
            body = kwargs.get('body')

            span.service = pin.service
            span.set_tag(metadata.METHOD, method)
            span.set_tag(metadata.URL, url)
            span.set_tag(metadata.PARAMS, urlencode(params))
            if config.elasticsearch.trace_query_string:
                span.set_tag(http.QUERY_STRING, urlencode(params))
            if method == 'GET':
                span.set_tag(metadata.BODY, instance.serializer.dumps(body))
            status = None

            # set analytics sample rate
            span.set_tag(
                ANALYTICS_SAMPLE_RATE_KEY,
                config.elasticsearch.get_analytics_sample_rate()
            )

            span = quantize(span)

            try:
                result = func(*args, **kwargs)
            except elasticsearch.exceptions.TransportError as e:
                span.set_tag(http.STATUS_CODE, getattr(e, 'status_code', 500))
                raise

            try:
                # Optional metadata extraction with soft fail.
                if isinstance(result, tuple) and len(result) == 2:
                    # elasticsearch<2.4; it returns both the status and the body
                    status, data = result
                else:
                    # elasticsearch>=2.4; internal change for ``Transport.perform_request``
                    # that just returns the body
                    data = result

                took = data.get('took')
                if took:
                    span.set_metric(metadata.TOOK, int(took))
            except Exception:
                pass

            if status:
                span.set_tag(http.STATUS_CODE, status)

            return result
    return _perform_request


# Backwards compatibility for anyone who decided to import `ddtrace.contrib.elasticsearch.patch._perform_request`
# DEV: `_perform_request` is a `wrapt.FunctionWrapper`
try:
    # DEV: Import as `es` to not shadow loop variables above
    import elasticsearch as es
    _perform_request = _get_perform_request(es)
except ImportError:
    pass
