import falcon

# pylint:disable=R0201,W0613,E0602


class HelloWorldResource:
    def _handle_request(self, _, resp):
        # pylint: disable=no-member
        resp.status = falcon.HTTP_201
        resp.body = "Hello World"

    def on_get(self, req, resp):
        self._handle_request(req, resp)

    def on_put(self, req, resp):
        self._handle_request(req, resp)

    def on_patch(self, req, resp):
        self._handle_request(req, resp)

    def on_post(self, req, resp):
        self._handle_request(req, resp)

    def on_delete(self, req, resp):
        self._handle_request(req, resp)

    def on_head(self, req, resp):
        self._handle_request(req, resp)


class ErrorResource:
    def on_get(self, req, resp):
        print(non_existent_var)  # noqa


def make_app():
    app = falcon.API()
    app.add_route("/hello", HelloWorldResource())
    app.add_route("/ping", HelloWorldResource())
    app.add_route("/error", ErrorResource())
    return app
