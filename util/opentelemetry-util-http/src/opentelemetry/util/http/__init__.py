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

from os import environ
from re import compile as re_compile
from re import search
from urllib.parse import urlparse, urlunparse


class ExcludeList:
    """Class to exclude certain paths (given as a list of regexes) from tracing requests"""

    def __init__(self, excluded_urls):
        self._excluded_urls = excluded_urls
        if self._excluded_urls:
            self._regex = re_compile("|".join(excluded_urls))

    def url_disabled(self, url: str) -> bool:
        return bool(self._excluded_urls and search(self._regex, url))


_root = r"OTEL_PYTHON_{}"


def get_traced_request_attrs(instrumentation):
    traced_request_attrs = environ.get(
        _root.format("{}_TRACED_REQUEST_ATTRS".format(instrumentation)), []
    )

    if traced_request_attrs:
        traced_request_attrs = [
            traced_request_attr.strip()
            for traced_request_attr in traced_request_attrs.split(",")
        ]

    return traced_request_attrs


def get_excluded_urls(instrumentation):
    excluded_urls = environ.get(
        _root.format("{}_EXCLUDED_URLS".format(instrumentation)), []
    )

    return parse_excluded_urls(excluded_urls)


def parse_excluded_urls(excluded_urls):
    """
    Small helper to put an arbitrary url list inside of ExcludeList
    """
    if excluded_urls:
        excluded_urls = [
            excluded_url.strip() for excluded_url in excluded_urls.split(",")
        ]

    return ExcludeList(excluded_urls)


def remove_url_credentials(url: str) -> str:
    """Given a string url, remove the username and password only if it is a valid url"""

    try:
        parsed = urlparse(url)
        if all([parsed.scheme, parsed.netloc]):  # checks for valid url
            parsed_url = urlparse(url)
            netloc = (
                (":".join(((parsed_url.hostname or ""), str(parsed_url.port))))
                if parsed_url.port
                else (parsed_url.hostname or "")
            )
            return urlunparse(
                (
                    parsed_url.scheme,
                    netloc,
                    parsed_url.path,
                    parsed_url.params,
                    parsed_url.query,
                    parsed_url.fragment,
                )
            )
    except ValueError:  # an unparseable url was passed
        pass
    return url
