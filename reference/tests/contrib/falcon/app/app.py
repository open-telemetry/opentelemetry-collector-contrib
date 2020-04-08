import falcon

from ddtrace.contrib.falcon import TraceMiddleware

from . import resources


def get_app(tracer=None, distributed_tracing=True):
    # initialize a traced Falcon application
    middleware = [TraceMiddleware(
        tracer, distributed_tracing=distributed_tracing)] if tracer else []
    app = falcon.API(middleware=middleware)

    # add resource routing
    app.add_route('/200', resources.Resource200())
    app.add_route('/201', resources.Resource201())
    app.add_route('/500', resources.Resource500())
    app.add_route('/exception', resources.ResourceException())
    app.add_route('/not_found', resources.ResourceNotFound())
    return app
