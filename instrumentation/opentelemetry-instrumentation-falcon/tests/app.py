import falcon
from packaging import version as package_version

# pylint:disable=R0201,W0613,E0602


class HelloWorldResource:
    def _handle_request(self, _, resp):
        # pylint: disable=no-member
        resp.status = falcon.HTTP_201

        _parsed_falcon_version = package_version.parse(falcon.__version__)
        if _parsed_falcon_version < package_version.parse("3.0.0"):
            # Falcon 1 and Falcon 2
            resp.body = "Hello World"
        else:
            resp.text = "Hello World"

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


class CustomResponseHeaderResource:
    def on_get(self, _, resp):
        # pylint: disable=no-member
        resp.status = falcon.HTTP_201
        resp.set_header("content-type", "text/plain; charset=utf-8")
        resp.set_header("content-length", "0")
        resp.set_header(
            "my-custom-header", "my-custom-value-1,my-custom-header-2"
        )
        resp.set_header("dont-capture-me", "test-value")
        resp.set_header(
            "my-custom-regex-header-1",
            "my-custom-regex-value-1,my-custom-regex-value-2",
        )
        resp.set_header(
            "My-Custom-Regex-Header-2",
            "my-custom-regex-value-3,my-custom-regex-value-4",
        )
        resp.set_header("my-secret-header", "my-secret-value")


def make_app():
    _parsed_falcon_version = package_version.parse(falcon.__version__)
    if _parsed_falcon_version < package_version.parse("3.0.0"):
        # Falcon 1 and Falcon 2
        app = falcon.API()
    else:
        # Falcon 3
        app = falcon.App()

    app.add_route("/hello", HelloWorldResource())
    app.add_route("/ping", HelloWorldResource())
    app.add_route("/error", ErrorResource())
    app.add_route(
        "/test_custom_response_headers", CustomResponseHeaderResource()
    )
    return app
