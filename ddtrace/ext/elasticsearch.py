from . import SpanTypes

# [TODO] Deprecated, remove when we remove AppTypes
TYPE = SpanTypes.ELASTICSEARCH
SERVICE = 'elasticsearch'
APP = 'elasticsearch'

# standard tags
URL = 'elasticsearch.url'
METHOD = 'elasticsearch.method'
TOOK = 'elasticsearch.took'
PARAMS = 'elasticsearch.params'
BODY = 'elasticsearch.body'
