import falcon


class Resource200(object):
    """Throw a handled exception here to ensure our use of
    set_traceback() doesn't affect 200s
    """
    def on_get(self, req, resp, **kwargs):
        try:
            1 / 0
        except Exception:
            pass

        resp.status = falcon.HTTP_200
        resp.body = 'Success'
        resp.append_header('my-response-header', 'my_response_value')


class Resource201(object):
    def on_post(self, req, resp, **kwargs):
        resp.status = falcon.HTTP_201
        resp.body = 'Success'


class Resource500(object):
    def on_get(self, req, resp, **kwargs):
        resp.status = falcon.HTTP_500
        resp.body = 'Failure'


class ResourceException(object):
    def on_get(self, req, resp, **kwargs):
        raise Exception('Ouch!')


class ResourceNotFound(object):
    def on_get(self, req, resp, **kwargs):
        # simulate that the endpoint is hit but raise a 404 because
        # the object isn't found in the database
        raise falcon.HTTPNotFound()
