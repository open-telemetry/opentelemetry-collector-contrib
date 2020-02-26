from ddtrace.contrib.pyramid import trace_pyramid

from pyramid.response import Response
from pyramid.config import Configurator
from pyramid.renderers import render_to_response
from pyramid.httpexceptions import (
    HTTPInternalServerError,
    HTTPFound,
    HTTPNotFound,
    HTTPNoContent,
)


def create_app(settings, instrument):
    """Return a pyramid wsgi app"""

    def index(request):
        return Response('idx')

    def error(request):
        raise HTTPInternalServerError('oh no')

    def exception(request):
        1 / 0

    def json(request):
        return {'a': 1}

    def renderer(request):
        return render_to_response('template.pt', {'foo': 'bar'}, request=request)

    def raise_redirect(request):
        raise HTTPFound()

    def raise_no_content(request):
        raise HTTPNoContent()

    def custom_exception_view(context, request):
        """Custom view that forces a HTTPException when no views
        are found to handle given request
        """
        if 'raise_exception' in request.url:
            raise HTTPNotFound()
        else:
            return HTTPNotFound()

    config = Configurator(settings=settings)
    config.add_route('index', '/')
    config.add_route('error', '/error')
    config.add_route('exception', '/exception')
    config.add_route('json', '/json')
    config.add_route('renderer', '/renderer')
    config.add_route('raise_redirect', '/redirect')
    config.add_route('raise_no_content', '/nocontent')
    config.add_view(index, route_name='index')
    config.add_view(error, route_name='error')
    config.add_view(exception, route_name='exception')
    config.add_view(json, route_name='json', renderer='json')
    config.add_view(renderer, route_name='renderer', renderer='template.pt')
    config.add_view(raise_redirect, route_name='raise_redirect')
    config.add_view(raise_no_content, route_name='raise_no_content')
    # required to reproduce a regression test
    config.add_notfound_view(custom_exception_view)
    # required for rendering tests
    renderer = config.testing_add_renderer('template.pt')

    if instrument:
        trace_pyramid(config)

    return config.make_wsgi_app(), renderer
