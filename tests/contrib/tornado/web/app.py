import os
import time

import tornado.web
import tornado.concurrent

from . import uimodules
from .compat import sleep, ThreadPoolExecutor


BASE_DIR = os.path.dirname(os.path.realpath(__file__))
STATIC_DIR = os.path.join(BASE_DIR, 'statics')


class SuccessHandler(tornado.web.RequestHandler):
    @tornado.gen.coroutine
    def get(self):
        self.write('OK')


class NestedHandler(tornado.web.RequestHandler):
    @tornado.gen.coroutine
    def get(self):
        tracer = self.settings['datadog_trace']['tracer']
        with tracer.trace('tornado.sleep'):
            yield sleep(0.05)
        self.write('OK')


class NestedWrapHandler(tornado.web.RequestHandler):
    @tornado.gen.coroutine
    def get(self):
        tracer = self.settings['datadog_trace']['tracer']

        # define a wrapped coroutine: having an inner coroutine
        # is only for easy testing
        @tracer.wrap('tornado.coro')
        @tornado.gen.coroutine
        def coro():
            yield sleep(0.05)

        yield coro()
        self.write('OK')


class NestedExceptionWrapHandler(tornado.web.RequestHandler):
    @tornado.gen.coroutine
    def get(self):
        tracer = self.settings['datadog_trace']['tracer']

        # define a wrapped coroutine: having an inner coroutine
        # is only for easy testing
        @tracer.wrap('tornado.coro')
        @tornado.gen.coroutine
        def coro():
            yield sleep(0.05)
            raise Exception('Ouch!')

        yield coro()
        self.write('OK')


class ExceptionHandler(tornado.web.RequestHandler):
    @tornado.gen.coroutine
    def get(self):
        raise Exception('Ouch!')


class HTTPExceptionHandler(tornado.web.RequestHandler):
    @tornado.gen.coroutine
    def get(self):
        raise tornado.web.HTTPError(status_code=501, log_message='unavailable', reason='Not Implemented')


class HTTPException500Handler(tornado.web.RequestHandler):
    @tornado.gen.coroutine
    def get(self):
        raise tornado.web.HTTPError(status_code=500, log_message='server error', reason='Server Error')


class TemplateHandler(tornado.web.RequestHandler):
    @tornado.gen.coroutine
    def get(self):
        self.render('templates/page.html', name='home')


class TemplatePartialHandler(tornado.web.RequestHandler):
    @tornado.gen.coroutine
    def get(self):
        self.render('templates/list.html', items=['python', 'go', 'ruby'])


class TemplateExceptionHandler(tornado.web.RequestHandler):
    @tornado.gen.coroutine
    def get(self):
        self.render('templates/exception.html')


class SyncSuccessHandler(tornado.web.RequestHandler):
    def get(self):
        self.write('OK')


class SyncExceptionHandler(tornado.web.RequestHandler):
    def get(self):
        raise Exception('Ouch!')


class SyncNestedWrapHandler(tornado.web.RequestHandler):
    def get(self):
        tracer = self.settings['datadog_trace']['tracer']

        # define a wrapped coroutine: having an inner coroutine
        # is only for easy testing
        @tracer.wrap('tornado.func')
        def func():
            time.sleep(0.05)

        func()
        self.write('OK')


class SyncNestedExceptionWrapHandler(tornado.web.RequestHandler):
    def get(self):
        tracer = self.settings['datadog_trace']['tracer']

        # define a wrapped coroutine: having an inner coroutine
        # is only for easy testing
        @tracer.wrap('tornado.func')
        def func():
            time.sleep(0.05)
            raise Exception('Ouch!')

        func()
        self.write('OK')


class CustomDefaultHandler(tornado.web.ErrorHandler):
    """
    Default handler that is used in case of 404 error; in our tests
    it's used only if defined in the get_app() function.
    """
    pass


class ExecutorHandler(tornado.web.RequestHandler):
    # used automatically by the @run_on_executor decorator
    executor = ThreadPoolExecutor(max_workers=3)

    @tornado.concurrent.run_on_executor
    def outer_executor(self):
        tracer = self.settings['datadog_trace']['tracer']
        with tracer.trace('tornado.executor.with'):
            time.sleep(0.05)

    @tornado.gen.coroutine
    def get(self):
        yield self.outer_executor()
        self.write('OK')


class ExecutorSubmitHandler(tornado.web.RequestHandler):
    executor = ThreadPoolExecutor(max_workers=3)

    def query(self):
        tracer = self.settings['datadog_trace']['tracer']
        with tracer.trace('tornado.executor.query'):
            time.sleep(0.05)

    @tornado.gen.coroutine
    def get(self):
        # run the query in another Executor, without using
        # Tornado decorators
        yield self.executor.submit(self.query)
        self.write('OK')


class ExecutorDelayedHandler(tornado.web.RequestHandler):
    # used automatically by the @run_on_executor decorator
    executor = ThreadPoolExecutor(max_workers=3)

    @tornado.concurrent.run_on_executor
    def outer_executor(self):
        # waiting here means expecting that the `get()` flushes
        # the request trace
        time.sleep(0.01)
        tracer = self.settings['datadog_trace']['tracer']
        with tracer.trace('tornado.executor.with'):
            time.sleep(0.05)

    @tornado.gen.coroutine
    def get(self):
        # we don't yield here but we expect that the outer_executor
        # has the right parent; tests that use this handler, must
        # yield sleep() to wait thread execution
        self.outer_executor()
        self.write('OK')


try:
    class ExecutorCustomHandler(tornado.web.RequestHandler):
        # not used automatically, a kwarg is required
        custom_thread_pool = ThreadPoolExecutor(max_workers=3)

        @tornado.concurrent.run_on_executor(executor='custom_thread_pool')
        def outer_executor(self):
            # wait before creating a trace so that we're sure
            # the `tornado.executor.with` span has the right
            # parent
            tracer = self.settings['datadog_trace']['tracer']
            with tracer.trace('tornado.executor.with'):
                time.sleep(0.05)

        @tornado.gen.coroutine
        def get(self):
            yield self.outer_executor()
            self.write('OK')
except TypeError:
    # the class definition fails because Tornado 4.0 and 4.1 don't support
    # `run_on_executor` with params. Because it's just this case, we can
    # use a try-except block, but if we have another case we may move
    # these endpoints outside the module and use a compatibility system
    class ExecutorCustomHandler(tornado.web.RequestHandler):
        pass


class ExecutorCustomArgsHandler(tornado.web.RequestHandler):
    @tornado.gen.coroutine
    def get(self):
        # this is not a legit use of the decorator so a failure is expected
        @tornado.concurrent.run_on_executor(object(), executor='_pool')
        def outer_executor(self):
            pass

        yield outer_executor(self)
        self.write('OK')


class ExecutorExceptionHandler(tornado.web.RequestHandler):
    # used automatically by the @run_on_executor decorator
    executor = ThreadPoolExecutor(max_workers=3)

    @tornado.concurrent.run_on_executor
    def outer_executor(self):
        # wait before creating a trace so that we're sure
        # the `tornado.executor.with` span has the right
        # parent
        time.sleep(0.05)
        tracer = self.settings['datadog_trace']['tracer']
        with tracer.trace('tornado.executor.with'):
            raise Exception('Ouch!')

    @tornado.gen.coroutine
    def get(self):
        yield self.outer_executor()
        self.write('OK')


class ExecutorWrapHandler(tornado.web.RequestHandler):
    # used automatically by the @run_on_executor decorator
    executor = ThreadPoolExecutor(max_workers=3)

    @tornado.gen.coroutine
    def get(self):
        tracer = self.settings['datadog_trace']['tracer']

        @tracer.wrap('tornado.executor.wrap')
        @tornado.concurrent.run_on_executor
        def outer_executor(self):
            time.sleep(0.05)

        yield outer_executor(self)
        self.write('OK')


class ExecutorExceptionWrapHandler(tornado.web.RequestHandler):
    # used automatically by the @run_on_executor decorator
    executor = ThreadPoolExecutor(max_workers=3)

    @tornado.gen.coroutine
    def get(self):
        tracer = self.settings['datadog_trace']['tracer']

        @tracer.wrap('tornado.executor.wrap')
        @tornado.concurrent.run_on_executor
        def outer_executor(self):
            time.sleep(0.05)
            raise Exception('Ouch!')

        yield outer_executor(self)
        self.write('OK')


def make_app(settings={}):
    """
    Create a Tornado web application, useful to test
    different behaviors.
    """
    settings['ui_modules'] = uimodules

    return tornado.web.Application([
        # custom handlers
        (r'/success/', SuccessHandler),
        (r'/nested/', NestedHandler),
        (r'/nested_wrap/', NestedWrapHandler),
        (r'/nested_exception_wrap/', NestedExceptionWrapHandler),
        (r'/exception/', ExceptionHandler),
        (r'/http_exception/', HTTPExceptionHandler),
        (r'/http_exception_500/', HTTPException500Handler),
        (r'/template/', TemplateHandler),
        (r'/template_partial/', TemplatePartialHandler),
        (r'/template_exception/', TemplateExceptionHandler),
        # handlers that spawn new threads
        (r'/executor_handler/', ExecutorHandler),
        (r'/executor_submit_handler/', ExecutorSubmitHandler),
        (r'/executor_delayed_handler/', ExecutorDelayedHandler),
        (r'/executor_custom_handler/', ExecutorCustomHandler),
        (r'/executor_custom_args_handler/', ExecutorCustomArgsHandler),
        (r'/executor_exception/', ExecutorExceptionHandler),
        (r'/executor_wrap_handler/', ExecutorWrapHandler),
        (r'/executor_wrap_exception/', ExecutorExceptionWrapHandler),
        # built-in handlers
        (r'/redirect/', tornado.web.RedirectHandler, {'url': '/success/'}),
        (r'/statics/(.*)', tornado.web.StaticFileHandler, {'path': STATIC_DIR}),
        # synchronous handlers
        (r'/sync_success/', SyncSuccessHandler),
        (r'/sync_exception/', SyncExceptionHandler),
        (r'/sync_nested_wrap/', SyncNestedWrapHandler),
        (r'/sync_nested_exception_wrap/', SyncNestedExceptionWrapHandler),
    ], **settings)
