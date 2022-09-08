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

import unittest
from http import HTTPStatus

from opentelemetry.instrumentation.sqlcommenter_utils import _add_sql_comment
from opentelemetry.instrumentation.utils import (
    _python_path_without_directory,
    http_status_to_status_code,
)
from opentelemetry.trace import StatusCode


class TestUtils(unittest.TestCase):
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

    def test_http_status_to_status_code_none(self):
        for status_code, expected in ((None, StatusCode.UNSET),):
            with self.subTest(status_code=status_code):
                actual = http_status_to_status_code(status_code)
                self.assertEqual(actual, expected, status_code)

    def test_http_status_to_status_code_redirect(self):
        for status_code, expected in (
            (HTTPStatus.MULTIPLE_CHOICES, StatusCode.ERROR),
            (HTTPStatus.MOVED_PERMANENTLY, StatusCode.ERROR),
            (HTTPStatus.TEMPORARY_REDIRECT, StatusCode.ERROR),
            (HTTPStatus.PERMANENT_REDIRECT, StatusCode.ERROR),
        ):
            with self.subTest(status_code=status_code):
                actual = http_status_to_status_code(
                    int(status_code), allow_redirect=False
                )
                self.assertEqual(actual, expected, status_code)

    def test_http_status_to_status_code_server(self):
        for status_code, expected in (
            (HTTPStatus.OK, StatusCode.UNSET),
            (HTTPStatus.ACCEPTED, StatusCode.UNSET),
            (HTTPStatus.IM_USED, StatusCode.UNSET),
            (HTTPStatus.MULTIPLE_CHOICES, StatusCode.UNSET),
            (HTTPStatus.BAD_REQUEST, StatusCode.UNSET),
            (HTTPStatus.UNAUTHORIZED, StatusCode.UNSET),
            (HTTPStatus.FORBIDDEN, StatusCode.UNSET),
            (HTTPStatus.NOT_FOUND, StatusCode.UNSET),
            (
                HTTPStatus.UNPROCESSABLE_ENTITY,
                StatusCode.UNSET,
            ),
            (
                HTTPStatus.TOO_MANY_REQUESTS,
                StatusCode.UNSET,
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
                actual = http_status_to_status_code(
                    int(status_code), server_span=True
                )
                self.assertEqual(actual, expected, status_code)

    def test_remove_current_directory_from_python_path_windows(self):
        directory = r"c:\users\Trayvon Martin\workplace\opentelemetry-python-contrib\opentelemetry-instrumentation\src\opentelemetry\instrumentation\auto_instrumentation"
        path_separator = r";"
        python_path = r"c:\users\Trayvon Martin\workplace\opentelemetry-python-contrib\opentelemetry-instrumentation\src\opentelemetry\instrumentation\auto_instrumentation;C:\Users\trayvonmartin\workplace"
        actual_python_path = _python_path_without_directory(
            python_path, directory, path_separator
        )
        expected_python_path = r"C:\Users\trayvonmartin\workplace"
        self.assertEqual(actual_python_path, expected_python_path)

    def test_remove_current_directory_from_python_path_linux(self):
        directory = r"/home/georgefloyd/workplace/opentelemetry-python-contrib/opentelemetry-instrumentation/src/opentelemetry/instrumentation/auto_instrumentation"
        path_separator = r":"
        python_path = r"/home/georgefloyd/workplace/opentelemetry-python-contrib/opentelemetry-instrumentation/src/opentelemetry/instrumentation/auto_instrumentation:/home/georgefloyd/workplace"
        actual_python_path = _python_path_without_directory(
            python_path, directory, path_separator
        )
        expected_python_path = r"/home/georgefloyd/workplace"
        self.assertEqual(actual_python_path, expected_python_path)

    def test_remove_current_directory_from_python_path_windows_only_path(self):
        directory = r"c:\users\Charleena Lyles\workplace\opentelemetry-python-contrib\opentelemetry-instrumentation\src\opentelemetry\instrumentation\auto_instrumentation"
        path_separator = r";"
        python_path = r"c:\users\Charleena Lyles\workplace\opentelemetry-python-contrib\opentelemetry-instrumentation\src\opentelemetry\instrumentation\auto_instrumentation"
        actual_python_path = _python_path_without_directory(
            python_path, directory, path_separator
        )
        self.assertEqual(actual_python_path, python_path)

    def test_remove_current_directory_from_python_path_linux_only_path(self):
        directory = r"/home/SandraBland/workplace/opentelemetry-python-contrib/opentelemetry-instrumentation/src/opentelemetry/instrumentation/auto_instrumentation"
        path_separator = r":"
        python_path = r"/home/SandraBland/workplace/opentelemetry-python-contrib/opentelemetry-instrumentation/src/opentelemetry/instrumentation/auto_instrumentation"
        actual_python_path = _python_path_without_directory(
            python_path, directory, path_separator
        )
        self.assertEqual(actual_python_path, python_path)

    def test_add_sql_comments_with_semicolon(self):
        sql_query_without_semicolon = "Select 1;"
        comments = {"comment_1": "value 1", "comment 2": "value 3"}
        commented_sql_without_semicolon = _add_sql_comment(
            sql_query_without_semicolon, **comments
        )

        self.assertEqual(
            commented_sql_without_semicolon,
            "Select 1 /*comment%%202='value%%203',comment_1='value%%201'*/;",
        )

    def test_add_sql_comments_without_semicolon(self):
        sql_query_without_semicolon = "Select 1"
        comments = {"comment_1": "value 1", "comment 2": "value 3"}
        commented_sql_without_semicolon = _add_sql_comment(
            sql_query_without_semicolon, **comments
        )

        self.assertEqual(
            commented_sql_without_semicolon,
            "Select 1 /*comment%%202='value%%203',comment_1='value%%201'*/",
        )

    def test_add_sql_comments_without_comments(self):
        sql_query_without_semicolon = "Select 1"
        comments = {}
        commented_sql_without_semicolon = _add_sql_comment(
            sql_query_without_semicolon, **comments
        )

        self.assertEqual(commented_sql_without_semicolon, "Select 1")
