import os

from mako.lookup import TemplateLookup

from pylons import config
from pylons.wsgiapp import PylonsApp

from routes.middleware import RoutesMiddleware
from beaker.middleware import SessionMiddleware, CacheMiddleware

from paste.registry import RegistryManager

from .router import create_routes
from .lib.helpers import AppGlobals


def make_app(global_conf, full_stack=True, **app_conf):
    # load Pylons environment
    root = os.path.dirname(os.path.abspath(__file__))
    paths = dict(
        templates=[os.path.join(root, 'templates')],
    )
    config.init_app(global_conf, app_conf, paths=paths)
    config['pylons.package'] = 'tests.contrib.pylons.app'
    config['pylons.app_globals'] = AppGlobals()

    # set Pylons routes
    config['routes.map'] = create_routes()

    # Create the Mako TemplateLookup, with the default auto-escaping
    config['pylons.app_globals'].mako_lookup = TemplateLookup(
        directories=paths['templates'],
    )

    # define a default middleware stack
    app = PylonsApp()
    app = RoutesMiddleware(app, config['routes.map'])
    app = SessionMiddleware(app, config)
    app = CacheMiddleware(app, config)
    app = RegistryManager(app)
    return app
