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
from unittest.mock import patch

from opentelemetry.util.http import get_excluded_urls


class TestGetExcludedUrls(unittest.TestCase):
    @patch.dict(
        "os.environ",
        {
            "OTEL_PYTHON_DJANGO_EXCLUDED_URLS": "excluded_arg/123,excluded_noarg"
        },
    )
    def test_config_from_instrumentation_env(self):
        exclude_list = get_excluded_urls("DJANGO")

        self.assertTrue(exclude_list.url_disabled("/excluded_arg/123"))
        self.assertTrue(exclude_list.url_disabled("/excluded_noarg"))
        self.assertFalse(exclude_list.url_disabled("/excluded_arg/125"))

    @patch.dict(
        "os.environ",
        {"OTEL_PYTHON_EXCLUDED_URLS": "excluded_arg/123,excluded_noarg"},
    )
    def test_config_from_generic_env(self):
        exclude_list = get_excluded_urls("DJANGO")

        self.assertTrue(exclude_list.url_disabled("/excluded_arg/123"))
        self.assertTrue(exclude_list.url_disabled("/excluded_noarg"))
        self.assertFalse(exclude_list.url_disabled("/excluded_arg/125"))

    @patch.dict(
        "os.environ",
        {
            "OTEL_PYTHON_DJANGO_EXCLUDED_URLS": "excluded_arg/123,excluded_noarg",
            "OTEL_PYTHON_EXCLUDED_URLS": "excluded_arg/125",
        },
    )
    def test_config_from_instrumentation_env_takes_precedence(self):
        exclude_list = get_excluded_urls("DJANGO")

        self.assertTrue(exclude_list.url_disabled("/excluded_arg/123"))
        self.assertTrue(exclude_list.url_disabled("/excluded_noarg"))
        self.assertFalse(exclude_list.url_disabled("/excluded_arg/125"))

    @patch.dict(
        "os.environ",
        {
            "OTEL_PYTHON_DJANGO_EXCLUDED_URLS": "",
            "OTEL_PYTHON_EXCLUDED_URLS": "excluded_arg/125",
        },
    )
    def test_config_from_instrumentation_env_empty(self):
        exclude_list = get_excluded_urls("DJANGO")

        self.assertFalse(exclude_list.url_disabled("/excluded_arg/123"))
        self.assertFalse(exclude_list.url_disabled("/excluded_noarg"))
        self.assertFalse(exclude_list.url_disabled("/excluded_arg/125"))
