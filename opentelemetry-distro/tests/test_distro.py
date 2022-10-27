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
# type: ignore

import os
from unittest import TestCase

from pkg_resources import DistributionNotFound, require

from opentelemetry.distro import OpenTelemetryDistro
from opentelemetry.environment_variables import (
    OTEL_METRICS_EXPORTER,
    OTEL_TRACES_EXPORTER,
)
from opentelemetry.sdk.environment_variables import OTEL_EXPORTER_OTLP_PROTOCOL


class TestDistribution(TestCase):
    def test_package_available(self):
        try:
            require(["opentelemetry-distro"])
        except DistributionNotFound:
            self.fail("opentelemetry-distro not installed")

    def test_default_configuration(self):
        distro = OpenTelemetryDistro()
        self.assertIsNone(os.environ.get(OTEL_TRACES_EXPORTER))
        self.assertIsNone(os.environ.get(OTEL_METRICS_EXPORTER))
        distro.configure()
        self.assertEqual(
            "otlp", os.environ.get(OTEL_TRACES_EXPORTER)
        )
        self.assertEqual(
            "otlp", os.environ.get(OTEL_METRICS_EXPORTER)
        )
        self.assertEqual(
            "grpc", os.environ.get(OTEL_EXPORTER_OTLP_PROTOCOL)
        )
