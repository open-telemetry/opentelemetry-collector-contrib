from webhelpers import *  # noqa


class ExceptionWithCodeMethod(Exception):
    """Use case where the status code is defined by
    the `code()` method.
    """
    def __init__(self, message):
        super(ExceptionWithCodeMethod, self).__init__(message)

    def code():
        pass


class AppGlobals(object):
    """Object used to store application globals."""
    pass


def get_render_fn():
    """Re-import the function everytime so that double-patching
    is correctly tested.
    """
    try:
        from pylons.templating import render_mako as render
    except ImportError:
        from pylons.templating import render

    return render
