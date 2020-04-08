import os
import jinja2
import asyncio
import aiohttp_jinja2

from aiohttp import web


BASE_DIR = os.path.dirname(os.path.realpath(__file__))
STATIC_DIR = os.path.join(BASE_DIR, 'statics')
TEMPLATE_DIR = os.path.join(BASE_DIR, 'templates')


@asyncio.coroutine
def home(request):
    return web.Response(text="What's tracing?")


@asyncio.coroutine
def name(request):
    name = request.match_info.get('name', 'Anonymous')
    return web.Response(text='Hello {}'.format(name))


@asyncio.coroutine
def coroutine_chaining(request):
    tracer = get_tracer(request)
    span = tracer.trace('aiohttp.coro_1')
    text = yield from coro_2(request)
    span.finish()
    return web.Response(text=text)


def route_exception(request):
    raise Exception('error')


@asyncio.coroutine
def route_async_exception(request):
    raise Exception('error')


@asyncio.coroutine
def route_wrapped_coroutine(request):
    tracer = get_tracer(request)

    @tracer.wrap('nested')
    @asyncio.coroutine
    def nested():
        yield from asyncio.sleep(0.25)

    yield from nested()
    return web.Response(text='OK')


@asyncio.coroutine
def route_sub_span(request):
    tracer = get_tracer(request)
    with tracer.trace('aiohttp.sub_span') as span:
        span.set_tag('sub_span', 'true')
        return web.Response(text='OK')


@asyncio.coroutine
def uncaught_server_error(request):
    return 1 / 0


@asyncio.coroutine
def caught_server_error(request):
    return web.Response(text='NOT OK', status=503)


@asyncio.coroutine
def coro_2(request):
    tracer = get_tracer(request)
    with tracer.trace('aiohttp.coro_2') as span:
        span.set_tag('aiohttp.worker', 'pending')
    return 'OK'


@asyncio.coroutine
def template_handler(request):
    return aiohttp_jinja2.render_template('template.jinja2', request, {'text': 'OK'})


@aiohttp_jinja2.template('template.jinja2')
@asyncio.coroutine
def template_decorator(request):
    return {'text': 'OK'}


@aiohttp_jinja2.template('error.jinja2')
@asyncio.coroutine
def template_error(request):
    return {}


@asyncio.coroutine
def delayed_handler(request):
    yield from asyncio.sleep(0.01)
    return web.Response(text='Done')


@asyncio.coroutine
def noop_middleware(app, handler):
    @asyncio.coroutine
    def middleware_handler(request):
        # noop middleware
        response = yield from handler(request)
        return response
    return middleware_handler


def setup_app(loop):
    """
    Use this method to create the app. It must receive
    the ``loop`` provided by the ``get_app`` method of
    ``AioHTTPTestCase`` class.
    """
    # configure the app
    app = web.Application(
        loop=loop,
        middlewares=[
            noop_middleware,
        ],
    )
    app.router.add_get('/', home)
    app.router.add_get('/delayed/', delayed_handler)
    app.router.add_get('/echo/{name}', name)
    app.router.add_get('/chaining/', coroutine_chaining)
    app.router.add_get('/exception', route_exception)
    app.router.add_get('/async_exception', route_async_exception)
    app.router.add_get('/wrapped_coroutine', route_wrapped_coroutine)
    app.router.add_get('/sub_span', route_sub_span)
    app.router.add_get('/uncaught_server_error', uncaught_server_error)
    app.router.add_get('/caught_server_error', caught_server_error)
    app.router.add_static('/statics', STATIC_DIR)
    # configure templates
    set_memory_loader(app)
    app.router.add_get('/template/', template_handler)
    app.router.add_get('/template_decorator/', template_decorator)
    app.router.add_get('/template_error/', template_error)

    return app


def set_memory_loader(app):
    aiohttp_jinja2.setup(app, loader=jinja2.DictLoader({
        'template.jinja2': '{{text}}',
        'error.jinja2': '{{1/0}}',
    }))


def set_filesystem_loader(app):
    aiohttp_jinja2.setup(app, loader=jinja2.FileSystemLoader(TEMPLATE_DIR))


def set_package_loader(app):
    aiohttp_jinja2.setup(app, loader=jinja2.PackageLoader('tests.contrib.aiohttp.app', 'templates'))


def get_tracer(request):
    """
    Utility function to retrieve the tracer from the given ``request``.
    It is meant to be used only for testing purposes.
    """
    return request['__datadog_request_span'].tracer
