# Copyright The OpenTelemetry Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
"""
Instrument `aio_pika` to trace RabbitMQ applications.

Usage
-----

* Start broker backend

.. code-block:: python

    docker run -p 5672:5672 rabbitmq

* Run instrumented task

.. code-block:: python

    import asyncio

    from aio_pika import Message, connect
    from opentelemetry.instrumentation.aio_pika import AioPikaInstrumentor

    AioPikaInstrumentor().instrument()


    async def main() -> None:
        connection = await connect("amqp://guest:guest@localhost/")
        async with connection:
            channel = await connection.channel()
            queue = await channel.declare_queue("hello")
            await channel.default_exchange.publish(
                Message(b"Hello World!"),
                routing_key=queue.name,
            )


    if __name__ == "__main__":
        asyncio.run(main())

API
---
"""
# pylint: disable=import-error

from .aio_pika_instrumentor import AioPikaInstrumentor
from .version import __version__

__all__ = ["AioPikaInstrumentor", "__version__"]
