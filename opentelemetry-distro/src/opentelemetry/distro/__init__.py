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

import os

from opentelemetry.environment_variables import (
    OTEL_METRICS_EXPORTER,
    OTEL_TRACES_EXPORTER,
)
from opentelemetry.instrumentation.distro import BaseDistro
from opentelemetry.sdk._configuration import _OTelSDKConfigurator
from opentelemetry.sdk.environment_variables import OTEL_EXPORTER_OTLP_PROTOCOL


class OpenTelemetryConfigurator(_OTelSDKConfigurator):
    pass


class OpenTelemetryDistro(BaseDistro):
    """
    The OpenTelemetry provided Distro configures a default set of
    configuration out of the box.
    """

    # pylint: disable=no-self-use
    def _configure(self, **kwargs):
        os.environ.setdefault(OTEL_TRACES_EXPORTER, "otlp")
        os.environ.setdefault(OTEL_METRICS_EXPORTER, "otlp")
        os.environ.setdefault(OTEL_EXPORTER_OTLP_PROTOCOL, "grpc")
