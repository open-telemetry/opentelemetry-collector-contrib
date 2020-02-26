from . import SpanTypes

# [TODO] Deprecated, remove when we remove AppTypes
# type of the spans
TYPE = SpanTypes.WORKER

SERVICE = 'kombu'

# net extension
VHOST = 'out.vhost'

# standard tags
EXCHANGE = 'kombu.exchange'
BODY_LEN = 'kombu.body_length'
ROUTING_KEY = 'kombu.routing_key'

PUBLISH_NAME = 'kombu.publish'
RECEIVE_NAME = 'kombu.receive'
