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

from opentelemetry.instrumentation.tortoiseorm import TortoiseORMInstrumentor
from opentelemetry.test.test_base import TestBase

# pylint: disable=too-many-public-methods


class TestTortoiseORMInstrumentor(TestBase):
    def setUp(self):
        super().setUp()
        TortoiseORMInstrumentor().instrument()

    def tearDown(self):
        super().tearDown()
        TortoiseORMInstrumentor().uninstrument()

    def test_tortoise(self):
        # FIXME This instrumentation has no tests at all and should have some
        # tests. This is being added just to make pytest not fail because no
        # tests are being collected for tortoise at the moment.
        pass
