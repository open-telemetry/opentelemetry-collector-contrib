import os

from routes import Mapper


def create_routes():
    """Change this function if you need to add more routes
    to your Pylons test app.
    """
    app_dir = os.path.dirname(os.path.abspath(__file__))
    controller_dir = os.path.join(app_dir, 'controllers')
    routes = Mapper(directory=controller_dir)
    routes.connect('/', controller='root', action='index')
    routes.connect('/raise_exception', controller='root', action='raise_exception')
    routes.connect('/raise_wrong_code', controller='root', action='raise_wrong_code')
    routes.connect('/raise_custom_code', controller='root', action='raise_custom_code')
    routes.connect('/raise_code_method', controller='root', action='raise_code_method')
    routes.connect('/render', controller='root', action='render')
    routes.connect('/render_exception', controller='root', action='render_exception')
    return routes
