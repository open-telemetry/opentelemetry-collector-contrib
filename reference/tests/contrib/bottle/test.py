import bottle
import ddtrace
import webtest

from tests.opentracer.utils import init_tracer
from ...base import BaseTracerTestCase
from ...utils import assert_span_http_status_code

from ddtrace import compat
from ddtrace.constants import ANALYTICS_SAMPLE_RATE_KEY
from ddtrace.contrib.bottle import TracePlugin
from ddtrace.ext import http

SERVICE = 'bottle-app'


class TraceBottleTest(BaseTracerTestCase):
    """
    Ensures that Bottle is properly traced.
    """
    def setUp(self):
        super(TraceBottleTest, self).setUp()

        # provide a dummy tracer
        self._original_tracer = ddtrace.tracer
        ddtrace.tracer = self.tracer
        # provide a Bottle app
        self.app = bottle.Bottle()

    def tearDown(self):
        # restore the tracer
        ddtrace.tracer = self._original_tracer

    def _trace_app(self, tracer=None):
        self.app.install(TracePlugin(service=SERVICE, tracer=tracer))
        self.app = webtest.TestApp(self.app)

    def test_200(self, query_string=''):
        if query_string:
            fqs = '?' + query_string
        else:
            fqs = ''

        # setup our test app
        @self.app.route('/hi/<name>')
        def hi(name):
            return 'hi %s' % name
        self._trace_app(self.tracer)

        # make a request
        resp = self.app.get('/hi/dougie' + fqs)
        assert resp.status_int == 200
        assert compat.to_unicode(resp.body) == u'hi dougie'
        # validate it's traced
        spans = self.tracer.writer.pop()
        assert len(spans) == 1
        s = spans[0]
        assert s.name == 'bottle.request'
        assert s.service == 'bottle-app'
        assert s.span_type == 'web'
        assert s.resource == 'GET /hi/<name>'
        assert_span_http_status_code(s, 200)
        assert s.get_tag('http.method') == 'GET'
        assert s.get_tag(http.URL) == 'http://localhost:80/hi/dougie'
        if ddtrace.config.bottle.trace_query_string:
            assert s.get_tag(http.QUERY_STRING) == query_string
        else:
            assert http.QUERY_STRING not in s.meta

        services = self.tracer.writer.pop_services()
        assert services == {}

    def test_query_string(self):
        return self.test_200('foo=bar')

    def test_query_string_multi_keys(self):
        return self.test_200('foo=bar&foo=baz&x=y')

    def test_query_string_trace(self):
        with self.override_http_config('bottle', dict(trace_query_string=True)):
            return self.test_200('foo=bar')

    def test_query_string_multi_keys_trace(self):
        with self.override_http_config('bottle', dict(trace_query_string=True)):
            return self.test_200('foo=bar&foo=baz&x=y')

    def test_2xx(self):
        @self.app.route('/2xx')
        def handled():
            return bottle.HTTPResponse("", status=202)
        self._trace_app(self.tracer)

        # make a request
        try:
            self.app.get('/2xx')
        except webtest.AppError:
            pass

        spans = self.tracer.writer.pop()
        assert len(spans) == 1
        s = spans[0]
        assert s.resource == 'GET /2xx'
        assert_span_http_status_code(s, 202)
        assert s.error == 0

    def test_400_return(self):
        @self.app.route('/400_return')
        def handled400():
            return bottle.HTTPResponse(status=400)
        self._trace_app(self.tracer)

        # make a request
        try:
            self.app.get('/400_return')
        except webtest.AppError:
            pass

        spans = self.tracer.writer.pop()
        assert len(spans) == 1
        s = spans[0]
        assert s.name == 'bottle.request'
        assert s.service == 'bottle-app'
        assert s.resource == 'GET /400_return'
        assert_span_http_status_code(s, 400)
        assert s.get_tag('http.method') == 'GET'
        assert s.get_tag(http.URL) == 'http://localhost:80/400_return'
        assert s.error == 0

    def test_400_raise(self):
        @self.app.route('/400_raise')
        def handled400():
            raise bottle.HTTPResponse(status=400)
        self._trace_app(self.tracer)

        # make a request
        try:
            self.app.get('/400_raise')
        except webtest.AppError:
            pass

        spans = self.tracer.writer.pop()
        assert len(spans) == 1
        s = spans[0]
        assert s.name == 'bottle.request'
        assert s.service == 'bottle-app'
        assert s.resource == 'GET /400_raise'
        assert_span_http_status_code(s, 400)
        assert s.get_tag('http.method') == 'GET'
        assert s.get_tag(http.URL) == 'http://localhost:80/400_raise'
        assert s.error == 1

    def test_500(self):
        @self.app.route('/hi')
        def hi():
            raise Exception('oh no')
        self._trace_app(self.tracer)

        # make a request
        try:
            self.app.get('/hi')
        except webtest.AppError:
            pass

        spans = self.tracer.writer.pop()
        assert len(spans) == 1
        s = spans[0]
        assert s.name == 'bottle.request'
        assert s.service == 'bottle-app'
        assert s.resource == 'GET /hi'
        assert_span_http_status_code(s, 500)
        assert s.get_tag('http.method') == 'GET'
        assert s.get_tag(http.URL) == 'http://localhost:80/hi'
        assert s.error == 1

    def test_5XX_response(self):
        """
        When a 5XX response is returned
            The span error attribute should be 1
        """
        @self.app.route('/5XX-1')
        def handled500_1():
            raise bottle.HTTPResponse(status=503)

        @self.app.route('/5XX-2')
        def handled500_2():
            raise bottle.HTTPError(status=502)

        @self.app.route('/5XX-3')
        def handled500_3():
            bottle.response.status = 503
            return 'hmmm'

        self._trace_app(self.tracer)

        try:
            self.app.get('/5XX-1')
        except webtest.AppError:
            pass
        spans = self.tracer.writer.pop()
        assert len(spans) == 1
        assert spans[0].error == 1

        try:
            self.app.get('/5XX-2')
        except webtest.AppError:
            pass
        spans = self.tracer.writer.pop()
        assert len(spans) == 1
        assert spans[0].error == 1

        try:
            self.app.get('/5XX-3')
        except webtest.AppError:
            pass
        spans = self.tracer.writer.pop()
        assert len(spans) == 1
        assert spans[0].error == 1

    def test_abort(self):
        @self.app.route('/hi')
        def hi():
            raise bottle.abort(420, 'Enhance Your Calm')
        self._trace_app(self.tracer)

        # make a request
        try:
            self.app.get('/hi')
        except webtest.AppError:
            pass

        spans = self.tracer.writer.pop()
        assert len(spans) == 1
        s = spans[0]
        assert s.name == 'bottle.request'
        assert s.service == 'bottle-app'
        assert s.resource == 'GET /hi'
        assert_span_http_status_code(s, 420)
        assert s.get_tag('http.method') == 'GET'
        assert s.get_tag(http.URL) == 'http://localhost:80/hi'

    def test_bottle_global_tracer(self):
        # without providing a Tracer instance, it should work
        @self.app.route('/home/')
        def home():
            return 'Hello world'
        self._trace_app()

        # make a request
        resp = self.app.get('/home/')
        assert resp.status_int == 200
        # validate it's traced
        spans = self.tracer.writer.pop()
        assert len(spans) == 1
        s = spans[0]
        assert s.name == 'bottle.request'
        assert s.service == 'bottle-app'
        assert s.resource == 'GET /home/'
        assert_span_http_status_code(s, 200)
        assert s.get_tag('http.method') == 'GET'
        assert s.get_tag(http.URL) == 'http://localhost:80/home/'

    def test_analytics_global_on_integration_default(self):
        """
        When making a request
            When an integration trace search is not event sample rate is not set and globally trace search is enabled
                We expect the root span to have the appropriate tag
        """
        # setup our test app
        @self.app.route('/hi/<name>')
        def hi(name):
            return 'hi %s' % name
        self._trace_app(self.tracer)

        with self.override_global_config(dict(analytics_enabled=True)):
            resp = self.app.get('/hi/dougie')
            assert resp.status_int == 200
            assert compat.to_unicode(resp.body) == u'hi dougie'

        root = self.get_root_span()
        root.assert_matches(
            name='bottle.request',
            metrics={
                ANALYTICS_SAMPLE_RATE_KEY: 1.0,
            },
        )

        for span in self.spans:
            if span == root:
                continue
            self.assertIsNone(span.get_metric(ANALYTICS_SAMPLE_RATE_KEY))

    def test_analytics_global_on_integration_on(self):
        """
        When making a request
            When an integration trace search is enabled and sample rate is set and globally trace search is enabled
                We expect the root span to have the appropriate tag
        """
        # setup our test app
        @self.app.route('/hi/<name>')
        def hi(name):
            return 'hi %s' % name
        self._trace_app(self.tracer)

        with self.override_global_config(dict(analytics_enabled=True)):
            with self.override_config('bottle', dict(analytics_enabled=True, analytics_sample_rate=0.5)):
                resp = self.app.get('/hi/dougie')
                assert resp.status_int == 200
                assert compat.to_unicode(resp.body) == u'hi dougie'

        root = self.get_root_span()
        root.assert_matches(
            name='bottle.request',
            metrics={
                ANALYTICS_SAMPLE_RATE_KEY: 0.5,
            },
        )

        for span in self.spans:
            if span == root:
                continue
            self.assertIsNone(span.get_metric(ANALYTICS_SAMPLE_RATE_KEY))

    def test_analytics_global_off_integration_default(self):
        """
        When making a request
            When an integration trace search is not set and sample rate is set and globally trace search is disabled
                We expect the root span to not include tag
        """
        # setup our test app
        @self.app.route('/hi/<name>')
        def hi(name):
            return 'hi %s' % name
        self._trace_app(self.tracer)

        with self.override_global_config(dict(analytics_enabled=False)):
            resp = self.app.get('/hi/dougie')
            assert resp.status_int == 200
            assert compat.to_unicode(resp.body) == u'hi dougie'

        root = self.get_root_span()
        self.assertIsNone(root.get_metric(ANALYTICS_SAMPLE_RATE_KEY))

        for span in self.spans:
            if span == root:
                continue
            self.assertIsNone(span.get_metric(ANALYTICS_SAMPLE_RATE_KEY))

    def test_analytics_global_off_integration_on(self):
        """
        When making a request
            When an integration trace search is enabled and sample rate is set and globally trace search is disabled
                We expect the root span to have the appropriate tag
        """
        # setup our test app
        @self.app.route('/hi/<name>')
        def hi(name):
            return 'hi %s' % name
        self._trace_app(self.tracer)

        with self.override_global_config(dict(analytics_enabled=False)):
            with self.override_config('bottle', dict(analytics_enabled=True, analytics_sample_rate=0.5)):
                resp = self.app.get('/hi/dougie')
                assert resp.status_int == 200
                assert compat.to_unicode(resp.body) == u'hi dougie'

        root = self.get_root_span()
        root.assert_matches(
            name='bottle.request',
            metrics={
                ANALYTICS_SAMPLE_RATE_KEY: 0.5,
            },
        )

        for span in self.spans:
            if span == root:
                continue
            self.assertIsNone(span.get_metric(ANALYTICS_SAMPLE_RATE_KEY))

    def test_200_ot(self):
        ot_tracer = init_tracer('my_svc', self.tracer)

        # setup our test app
        @self.app.route('/hi/<name>')
        def hi(name):
            return 'hi %s' % name
        self._trace_app(self.tracer)

        # make a request
        with ot_tracer.start_active_span('ot_span'):
            resp = self.app.get('/hi/dougie')

        assert resp.status_int == 200
        assert compat.to_unicode(resp.body) == u'hi dougie'
        # validate it's traced
        spans = self.tracer.writer.pop()
        assert len(spans) == 2
        ot_span, dd_span = spans

        # confirm the parenting
        assert ot_span.parent_id is None
        assert dd_span.parent_id == ot_span.span_id

        assert ot_span.resource == 'ot_span'

        assert dd_span.name == 'bottle.request'
        assert dd_span.service == 'bottle-app'
        assert dd_span.resource == 'GET /hi/<name>'
        assert_span_http_status_code(dd_span, 200)
        assert dd_span.get_tag('http.method') == 'GET'
        assert dd_span.get_tag(http.URL) == 'http://localhost:80/hi/dougie'

        services = self.tracer.writer.pop_services()
        assert services == {}
