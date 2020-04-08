# -*- coding: utf-8 -*-
import os

from flask import Flask, render_template


class TestError(Exception):
    pass


class HandleMe(Exception):
    pass


def create_app():
    """Initializes a new Flask application. This method is required to
    be sure each time a test is executed, the Flask app is always new
    and without any tracing side effect from the previous execution.
    """
    cur_dir = os.path.dirname(os.path.realpath(__file__))
    tmpl_path = os.path.join(cur_dir, 'test_templates')
    app = Flask(__name__, template_folder=tmpl_path)

    @app.route('/')
    def index():
        return 'hello'

    @app.route('/error')
    def error():
        raise TestError()

    @app.route('/handleme')
    def handle_me():
        raise HandleMe()

    @app.route('/fatal')
    def fatal():
        1 / 0

    @app.route('/tmpl')
    def tmpl():
        return render_template('test.html', world='earth')

    @app.route('/tmpl/err')
    def tmpl_err():
        return render_template('err.html')

    @app.route('/tmpl/render_err')
    def tmpl_render_err():
        return render_template('render_err.html')

    @app.route('/child')
    def child():
        with app._tracer.trace('child') as span:
            span.set_tag('a', 'b')
            return 'child'

    @app.route('/custom_span')
    def custom_span():
        span = app._tracer.current_span()
        assert span
        span.resource = 'overridden'
        return 'hiya'

    def unicode_view():
        return u'üŋïĉóđē'

    app.add_url_rule(
        u'/üŋïĉóđē',
        u'üŋïĉóđē',
        unicode_view,
    )

    @app.errorhandler(TestError)
    def handle_my_exception(e):
        assert isinstance(e, TestError)
        return 'error', 500

    @app.errorhandler(HandleMe)
    def err_to_202(e):
        assert isinstance(e, HandleMe)
        return 'handled', 202

    return app
