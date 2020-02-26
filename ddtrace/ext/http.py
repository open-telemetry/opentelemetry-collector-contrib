"""
Standard http tags.

For example:

span.set_tag(URL, '/user/home')
span.set_tag(STATUS_CODE, 404)
"""
from . import SpanTypes

# [TODO] Deprecated, remove when we remove AppTypes
# type of the spans
TYPE = SpanTypes.HTTP

# tags
URL = 'http.url'
METHOD = 'http.method'
STATUS_CODE = 'http.status_code'
QUERY_STRING = 'http.query.string'

# template render span type
TEMPLATE = 'template'


def normalize_status_code(code):
    return code.split(' ')[0]
