from argparse import Namespace

from yarl import URL

MESSAGE_ID = "meesage_id"
CORRELATION_ID = "correlation_id"
MESSAGING_SYSTEM = "rabbitmq"
EXCHANGE_NAME = "exchange_name"
QUEUE_NAME = "queue_name"
ROUTING_KEY = "routing_key"
SERVER_HOST = "localhost"
SERVER_PORT = 1234
SERVER_USER = "guest"
SERVER_PASS = "guest"
SERVER_URL = URL(
    f"amqp://{SERVER_USER}:{SERVER_PASS}@{SERVER_HOST}:{SERVER_PORT}/"
)
CONNECTION = Namespace(connection=Namespace(url=SERVER_URL))
CHANNEL = Namespace(connection=CONNECTION, loop=None)
MESSAGE = Namespace(
    properties=Namespace(
        message_id=MESSAGE_ID, correlation_id=CORRELATION_ID, headers={}
    ),
    exchange=EXCHANGE_NAME,
    headers={},
)
