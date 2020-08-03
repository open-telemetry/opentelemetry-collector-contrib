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

from werkzeug.test import Client
from werkzeug.wrappers import BaseResponse

from opentelemetry.configuration import Configuration


class InstrumentationTest:
    def setUp(self):  # pylint: disable=invalid-name
        super().setUp()  # pylint: disable=no-member
        Configuration._reset()  # pylint: disable=protected-access

    @staticmethod
    def _hello_endpoint(helloid):
        if helloid == 500:
            raise ValueError(":-(")
        return "Hello: " + str(helloid)

    def _common_initialization(self):
        def excluded_endpoint():
            return "excluded"

        def excluded2_endpoint():
            return "excluded2"

        # pylint: disable=no-member
        self.app.route("/hello/<int:helloid>")(self._hello_endpoint)
        self.app.route("/excluded/<int:helloid>")(self._hello_endpoint)
        self.app.route("/excluded")(excluded_endpoint)
        self.app.route("/excluded2")(excluded2_endpoint)

        # pylint: disable=attribute-defined-outside-init
        self.client = Client(self.app, BaseResponse)
