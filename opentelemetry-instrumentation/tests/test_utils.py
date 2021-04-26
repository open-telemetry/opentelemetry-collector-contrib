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

from http import HTTPStatus

from opentelemetry.instrumentation.utils import http_status_to_status_code
from opentelemetry.test.test_base import TestBase
from opentelemetry.trace import StatusCode


class TestUtils(TestBase):
    # See https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/trace/semantic_conventions/http.md#status
    def test_http_status_to_status_code(self):
        for status_code, expected in (
            (HTTPStatus.OK, StatusCode.UNSET),
            (HTTPStatus.ACCEPTED, StatusCode.UNSET),
            (HTTPStatus.IM_USED, StatusCode.UNSET),
            (HTTPStatus.MULTIPLE_CHOICES, StatusCode.UNSET),
            (HTTPStatus.BAD_REQUEST, StatusCode.ERROR),
            (HTTPStatus.UNAUTHORIZED, StatusCode.ERROR),
            (HTTPStatus.FORBIDDEN, StatusCode.ERROR),
            (HTTPStatus.NOT_FOUND, StatusCode.ERROR),
            (
                HTTPStatus.UNPROCESSABLE_ENTITY,
                StatusCode.ERROR,
            ),
            (
                HTTPStatus.TOO_MANY_REQUESTS,
                StatusCode.ERROR,
            ),
            (HTTPStatus.NOT_IMPLEMENTED, StatusCode.ERROR),
            (HTTPStatus.SERVICE_UNAVAILABLE, StatusCode.ERROR),
            (
                HTTPStatus.GATEWAY_TIMEOUT,
                StatusCode.ERROR,
            ),
            (
                HTTPStatus.HTTP_VERSION_NOT_SUPPORTED,
                StatusCode.ERROR,
            ),
            (600, StatusCode.ERROR),
            (99, StatusCode.ERROR),
        ):
            with self.subTest(status_code=status_code):
                actual = http_status_to_status_code(int(status_code))
                self.assertEqual(actual, expected, status_code)
