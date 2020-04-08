from ddtrace import config
from ddtrace.constants import ANALYTICS_SAMPLE_RATE_KEY
from ddtrace.ext import errors as errx, http as httpx

from tests.opentracer.utils import init_tracer
from ...utils import assert_span_http_status_code


class FalconTestCase(object):
    """Falcon mixin test case that includes all possible tests. If you need
    to add new tests, add them here so that they're shared across manual
    and automatic instrumentation.
    """
    def test_404(self):
        out = self.simulate_get('/fake_endpoint')
        assert out.status_code == 404

        traces = self.tracer.writer.pop_traces()
        assert len(traces) == 1
        assert len(traces[0]) == 1
        span = traces[0][0]
        assert span.name == 'falcon.request'
        assert span.service == self._service
        assert span.resource == 'GET 404'
        assert_span_http_status_code(span, 404)
        assert span.get_tag(httpx.URL) == 'http://falconframework.org/fake_endpoint'
        assert httpx.QUERY_STRING not in span.meta
        assert span.parent_id is None

    def test_exception(self):
        try:
            self.simulate_get('/exception')
        except Exception:
            pass
        else:
            assert 0

        traces = self.tracer.writer.pop_traces()
        assert len(traces) == 1
        assert len(traces[0]) == 1
        span = traces[0][0]
        assert span.name == 'falcon.request'
        assert span.service == self._service
        assert span.resource == 'GET tests.contrib.falcon.app.resources.ResourceException'
        assert_span_http_status_code(span, 500)
        assert span.get_tag(httpx.URL) == 'http://falconframework.org/exception'
        assert span.parent_id is None

    def test_200(self, query_string=''):
        out = self.simulate_get('/200', query_string=query_string)
        assert out.status_code == 200
        assert out.content.decode('utf-8') == 'Success'

        traces = self.tracer.writer.pop_traces()
        assert len(traces) == 1
        assert len(traces[0]) == 1
        span = traces[0][0]
        assert span.name == 'falcon.request'
        assert span.service == self._service
        assert span.resource == 'GET tests.contrib.falcon.app.resources.Resource200'
        assert_span_http_status_code(span, 200)
        fqs = ('?' + query_string) if query_string else ''
        assert span.get_tag(httpx.URL) == 'http://falconframework.org/200' + fqs
        if config.falcon.trace_query_string:
            assert span.get_tag(httpx.QUERY_STRING) == query_string
        else:
            assert httpx.QUERY_STRING not in span.meta
        assert span.parent_id is None
        assert span.span_type == 'web'

    def test_200_qs(self):
        return self.test_200('foo=bar')

    def test_200_multi_qs(self):
        return self.test_200('foo=bar&foo=baz&x=y')

    def test_200_qs_trace(self):
        with self.override_http_config('falcon', dict(trace_query_string=True)):
            return self.test_200('foo=bar')

    def test_200_multi_qs_trace(self):
        with self.override_http_config('falcon', dict(trace_query_string=True)):
            return self.test_200('foo=bar&foo=baz&x=y')

    def test_analytics_global_on_integration_default(self):
        """
        When making a request
            When an integration trace search is not event sample rate is not set and globally trace search is enabled
                We expect the root span to have the appropriate tag
        """
        with self.override_global_config(dict(analytics_enabled=True)):
            out = self.simulate_get('/200')
            self.assertEqual(out.status_code, 200)
            self.assertEqual(out.content.decode('utf-8'), 'Success')

            self.assert_structure(
                dict(name='falcon.request', metrics={ANALYTICS_SAMPLE_RATE_KEY: 1.0})
            )

    def test_analytics_global_on_integration_on(self):
        """
        When making a request
            When an integration trace search is enabled and sample rate is set and globally trace search is enabled
                We expect the root span to have the appropriate tag
        """
        with self.override_global_config(dict(analytics_enabled=True)):
            with self.override_config('falcon', dict(analytics_enabled=True, analytics_sample_rate=0.5)):
                out = self.simulate_get('/200')
                self.assertEqual(out.status_code, 200)
                self.assertEqual(out.content.decode('utf-8'), 'Success')

                self.assert_structure(
                    dict(name='falcon.request', metrics={ANALYTICS_SAMPLE_RATE_KEY: 0.5})
                )

    def test_analytics_global_off_integration_default(self):
        """
        When making a request
            When an integration trace search is not set and sample rate is set and globally trace search is disabled
                We expect the root span to not include tag
        """
        with self.override_global_config(dict(analytics_enabled=False)):
            out = self.simulate_get('/200')
            self.assertEqual(out.status_code, 200)
            self.assertEqual(out.content.decode('utf-8'), 'Success')

            root = self.get_root_span()
            self.assertIsNone(root.get_metric(ANALYTICS_SAMPLE_RATE_KEY))

    def test_analytics_global_off_integration_on(self):
        """
        When making a request
            When an integration trace search is enabled and sample rate is set and globally trace search is disabled
                We expect the root span to have the appropriate tag
        """
        with self.override_global_config(dict(analytics_enabled=False)):
            with self.override_config('falcon', dict(analytics_enabled=True, analytics_sample_rate=0.5)):
                out = self.simulate_get('/200')
                self.assertEqual(out.status_code, 200)
                self.assertEqual(out.content.decode('utf-8'), 'Success')

                self.assert_structure(
                    dict(name='falcon.request', metrics={ANALYTICS_SAMPLE_RATE_KEY: 0.5})
                )

    def test_201(self):
        out = self.simulate_post('/201')
        assert out.status_code == 201
        assert out.content.decode('utf-8') == 'Success'

        traces = self.tracer.writer.pop_traces()
        assert len(traces) == 1
        assert len(traces[0]) == 1
        span = traces[0][0]
        assert span.name == 'falcon.request'
        assert span.service == self._service
        assert span.resource == 'POST tests.contrib.falcon.app.resources.Resource201'
        assert_span_http_status_code(span, 201)
        assert span.get_tag(httpx.URL) == 'http://falconframework.org/201'
        assert span.parent_id is None

    def test_500(self):
        out = self.simulate_get('/500')
        assert out.status_code == 500
        assert out.content.decode('utf-8') == 'Failure'

        traces = self.tracer.writer.pop_traces()
        assert len(traces) == 1
        assert len(traces[0]) == 1
        span = traces[0][0]
        assert span.name == 'falcon.request'
        assert span.service == self._service
        assert span.resource == 'GET tests.contrib.falcon.app.resources.Resource500'
        assert_span_http_status_code(span, 500)
        assert span.get_tag(httpx.URL) == 'http://falconframework.org/500'
        assert span.parent_id is None

    def test_404_exception(self):
        out = self.simulate_get('/not_found')
        assert out.status_code == 404

        traces = self.tracer.writer.pop_traces()
        assert len(traces) == 1
        assert len(traces[0]) == 1
        span = traces[0][0]
        assert span.name == 'falcon.request'
        assert span.service == self._service
        assert span.resource == 'GET tests.contrib.falcon.app.resources.ResourceNotFound'
        assert_span_http_status_code(span, 404)
        assert span.get_tag(httpx.URL) == 'http://falconframework.org/not_found'
        assert span.parent_id is None

    def test_404_exception_no_stacktracer(self):
        # it should not have the stacktrace when a 404 exception is raised
        out = self.simulate_get('/not_found')
        assert out.status_code == 404

        traces = self.tracer.writer.pop_traces()
        assert len(traces) == 1
        assert len(traces[0]) == 1
        span = traces[0][0]
        assert span.name == 'falcon.request'
        assert span.service == self._service
        assert_span_http_status_code(span, 404)
        assert span.get_tag(errx.ERROR_TYPE) is None
        assert span.parent_id is None

    def test_200_ot(self):
        """OpenTracing version of test_200."""
        ot_tracer = init_tracer('my_svc', self.tracer)

        with ot_tracer.start_active_span('ot_span'):
            out = self.simulate_get('/200')

        assert out.status_code == 200
        assert out.content.decode('utf-8') == 'Success'

        traces = self.tracer.writer.pop_traces()
        assert len(traces) == 1
        assert len(traces[0]) == 2
        ot_span, dd_span = traces[0]

        # confirm the parenting
        assert ot_span.parent_id is None
        assert dd_span.parent_id == ot_span.span_id

        assert ot_span.service == 'my_svc'
        assert ot_span.resource == 'ot_span'

        assert dd_span.name == 'falcon.request'
        assert dd_span.service == self._service
        assert dd_span.resource == 'GET tests.contrib.falcon.app.resources.Resource200'
        assert_span_http_status_code(dd_span, 200)
        assert dd_span.get_tag(httpx.URL) == 'http://falconframework.org/200'

    def test_falcon_request_hook(self):
        @config.falcon.hooks.on('request')
        def on_falcon_request(span, request, response):
            span.set_tag('my.custom', 'tag')

        out = self.simulate_get('/200')
        assert out.status_code == 200
        assert out.content.decode('utf-8') == 'Success'

        traces = self.tracer.writer.pop_traces()
        assert len(traces) == 1
        assert len(traces[0]) == 1
        span = traces[0][0]
        assert span.get_tag('http.request.headers.my_header') is None
        assert span.get_tag('http.response.headers.my_response_header') is None

        assert span.name == 'falcon.request'

        assert span.get_tag('my.custom') == 'tag'

    def test_http_header_tracing(self):
        with self.override_config('falcon', {}):
            config.falcon.http.trace_headers(['my-header', 'my-response-header'])
            self.simulate_get('/200', headers={'my-header': 'my_value'})
            traces = self.tracer.writer.pop_traces()

        assert len(traces) == 1
        assert len(traces[0]) == 1
        span = traces[0][0]
        assert span.get_tag('http.request.headers.my-header') == 'my_value'
        assert span.get_tag('http.response.headers.my-response-header') == 'my_response_value'
