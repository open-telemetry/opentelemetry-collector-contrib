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

from opentelemetry.configuration import Configuration


class InstrumentationTest:
    def setUp(self):  # pylint: disable=invalid-name
        super().setUp()  # pylint: disable=no-member
        Configuration._reset()  # pylint: disable=protected-access

    @staticmethod
    def _hello_endpoint(request):
        helloid = int(request.matchdict["helloid"])
        if helloid == 500:
            raise exc.HTTPInternalServerError()
        return Response("Hello: " + str(helloid))

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

        # pylint: disable=attribute-defined-outside-init
        self.client = Client(config.make_wsgi_app(), BaseResponse)
