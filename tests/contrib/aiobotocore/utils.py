import aiobotocore.session

from ddtrace import Pin
from contextlib import contextmanager


LOCALSTACK_ENDPOINT_URL = {
    's3': 'http://localhost:5000',
    'ec2': 'http://localhost:5001',
    'kms': 'http://localhost:5002',
    'sqs': 'http://localhost:5003',
    'lambda': 'http://localhost:5004',
    'kinesis': 'http://localhost:5005',
}


@contextmanager
def aiobotocore_client(service, tracer):
    """Helper function that creates a new aiobotocore client so that
    it is closed at the end of the context manager.
    """
    session = aiobotocore.session.get_session()
    endpoint = LOCALSTACK_ENDPOINT_URL[service]
    client = session.create_client(
        service,
        region_name='us-west-2',
        endpoint_url=endpoint,
        aws_access_key_id='aws',
        aws_secret_access_key='aws',
        aws_session_token='aws',
    )
    Pin.override(client, tracer=tracer)
    try:
        yield client
    finally:
        client.close()
