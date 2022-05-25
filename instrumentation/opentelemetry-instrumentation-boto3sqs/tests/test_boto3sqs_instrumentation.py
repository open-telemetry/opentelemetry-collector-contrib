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

# pylint: disable=no-name-in-module

from unittest import TestCase

import boto3
import botocore.client
from wrapt import BoundFunctionWrapper, FunctionWrapper

from opentelemetry.instrumentation.boto3sqs import (
    _OPENTELEMETRY_ATTRIBUTE_IDENTIFIER,
    Boto3SQSGetter,
    Boto3SQSInstrumentor,
    Boto3SQSSetter,
)


# pylint: disable=attribute-defined-outside-init
class TestBoto3SQSInstrumentor(TestCase):
    def define_sqs_mock(self) -> None:
        # pylint: disable=R0201
        class SQSClientMock(botocore.client.BaseClient):
            def send_message(self, *args, **kwargs):
                ...

            def send_message_batch(self, *args, **kwargs):
                ...

            def receive_message(self, *args, **kwargs):
                ...

            def delete_message(self, *args, **kwargs):
                ...

            def delete_message_batch(self, *args, **kwargs):
                ...

        self._boto_sqs_mock = SQSClientMock

    def test_instrument_api_before_client_init(self) -> None:
        instrumentation = Boto3SQSInstrumentor()

        instrumentation.instrument()
        self.assertTrue(isinstance(boto3.client, FunctionWrapper))
        instrumentation.uninstrument()

    def test_instrument_api_after_client_init(self) -> None:
        self.define_sqs_mock()
        instrumentation = Boto3SQSInstrumentor()

        instrumentation.instrument()
        self.assertTrue(isinstance(boto3.client, FunctionWrapper))
        self.assertTrue(
            isinstance(self._boto_sqs_mock.send_message, BoundFunctionWrapper)
        )
        self.assertTrue(
            isinstance(
                self._boto_sqs_mock.send_message_batch, BoundFunctionWrapper
            )
        )
        self.assertTrue(
            isinstance(
                self._boto_sqs_mock.receive_message, BoundFunctionWrapper
            )
        )
        self.assertTrue(
            isinstance(
                self._boto_sqs_mock.delete_message, BoundFunctionWrapper
            )
        )
        self.assertTrue(
            isinstance(
                self._boto_sqs_mock.delete_message_batch, BoundFunctionWrapper
            )
        )
        instrumentation.uninstrument()


class TestBoto3SQSGetter(TestCase):
    def setUp(self) -> None:
        self.getter = Boto3SQSGetter()

    def test_get_none(self) -> None:
        carrier = {}
        value = self.getter.get(carrier, "test")
        self.assertIsNone(value)

    def test_get_value(self) -> None:
        key = "test"
        value = "value"
        carrier = {
            f"{_OPENTELEMETRY_ATTRIBUTE_IDENTIFIER}{key}": {
                "StringValue": value,
                "DataType": "String",
            }
        }
        val = self.getter.get(carrier, key)
        self.assertEqual(val, [value])

    def test_keys(self):
        key1 = "test1"
        value1 = "value1"
        key2 = "test2"
        value2 = "value2"
        carrier = {
            f"{_OPENTELEMETRY_ATTRIBUTE_IDENTIFIER}{key1}": {
                "StringValue": value1,
                "DataType": "String",
            },
            key2: {"StringValue": value2, "DataType": "String"},
        }
        keys = self.getter.keys(carrier)
        self.assertEqual(keys, [key1, key2])

    def test_keys_empty(self):
        keys = self.getter.keys({})
        self.assertEqual(keys, [])


class TestBoto3SQSSetter(TestCase):
    def setUp(self) -> None:
        self.setter = Boto3SQSSetter()

    def test_simple(self):
        original_key = "SomeHeader"
        original_value = {"NumberValue": 1, "DataType": "Number"}
        carrier = {original_key: original_value.copy()}
        key = "test"
        value = "value"
        self.setter.set(carrier, key, value)
        # Ensure the original value is not harmed
        for dict_key, dict_val in carrier[original_key].items():
            self.assertEqual(original_value[dict_key], dict_val)
        # Ensure the new key is added well
        self.assertIn(
            f"{_OPENTELEMETRY_ATTRIBUTE_IDENTIFIER}{key}", carrier.keys()
        )
        new_value = carrier[f"{_OPENTELEMETRY_ATTRIBUTE_IDENTIFIER}{key}"]
        self.assertEqual(new_value["StringValue"], value)
