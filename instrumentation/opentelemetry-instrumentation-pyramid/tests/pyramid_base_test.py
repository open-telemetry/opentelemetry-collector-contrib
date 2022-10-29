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

import pyramid.httpexceptions as exc
from pyramid.response import Response
from werkzeug.test import Client
from werkzeug.wrappers import BaseResponse


class InstrumentationTest:
    @staticmethod
    def _hello_endpoint(request):
        helloid = int(request.matchdict["helloid"])
        if helloid == 500:
            raise exc.HTTPInternalServerError()
        if helloid == 404:
            raise exc.HTTPNotFound()
        if helloid == 302:
            raise exc.HTTPFound()
        if helloid == 204:
            raise exc.HTTPNoContent()
        if helloid == 900:
            raise NotImplementedError()
        return Response("Hello: " + str(helloid))

    @staticmethod
    def _custom_response_header_endpoint(request):
        headers = {
            "content-type": "text/plain; charset=utf-8",
            "content-length": "7",
            "my-custom-header": "my-custom-value-1,my-custom-header-2",
            "my-custom-regex-header-1": "my-custom-regex-value-1,my-custom-regex-value-2",
            "My-Custom-Regex-Header-2": "my-custom-regex-value-3,my-custom-regex-value-4",
            "my-secret-header": "my-secret-value",
            "dont-capture-me": "test-value",
        }
        return Response("Testing", headers=headers)

    def _common_initialization(self, config):
        # pylint: disable=unused-argument
        def excluded_endpoint(request):
            return Response("excluded")

        # pylint: disable=unused-argument
        def excluded2_endpoint(request):
            return Response("excluded2")

        config.add_route("hello", "/hello/{helloid}")
        config.add_view(self._hello_endpoint, route_name="hello")
        config.add_route("excluded_arg", "/excluded/{helloid}")
        config.add_view(self._hello_endpoint, route_name="excluded_arg")
        config.add_route("excluded", "/excluded_noarg")
        config.add_view(excluded_endpoint, route_name="excluded")
        config.add_route("excluded2", "/excluded_noarg2")
        config.add_view(excluded2_endpoint, route_name="excluded2")
        config.add_route(
            "custom_response_headers", "/test_custom_response_headers"
        )
        config.add_view(
            self._custom_response_header_endpoint,
            route_name="custom_response_headers",
        )

        # pylint: disable=attribute-defined-outside-init
        self.client = Client(config.make_wsgi_app(), BaseResponse)
