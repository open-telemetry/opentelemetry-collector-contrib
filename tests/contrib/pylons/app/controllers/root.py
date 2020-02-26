from pylons.controllers import WSGIController

from ..lib.helpers import ExceptionWithCodeMethod, get_render_fn


class BaseController(WSGIController):

    def __call__(self, environ, start_response):
        """Invoke the Controller"""
        # WSGIController.__call__ dispatches to the Controller method
        # the request is routed to. This routing information is
        # available in environ['pylons.routes_dict']
        return WSGIController.__call__(self, environ, start_response)


class RootController(BaseController):
    """Controller used for most tests"""

    def index(self):
        return 'Hello World'

    def raise_exception(self):
        raise Exception('Ouch!')

    def raise_wrong_code(self):
        e = Exception('Ouch!')
        e.code = 'wrong formatted code'
        raise e

    def raise_code_method(self):
        raise ExceptionWithCodeMethod('Ouch!')

    def raise_custom_code(self):
        e = Exception('Ouch!')
        e.code = '512'
        raise e

    def render(self):
        render = get_render_fn()
        return render('/template.mako')

    def render_exception(self):
        render = get_render_fn()
        return render('/exception.mako')
