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

"""
OpenTelemetry Base Configurator
"""

from abc import ABC, abstractmethod
from logging import getLogger

_LOG = getLogger(__name__)


class BaseConfigurator(ABC):
    """An ABC for configurators

    Configurators are used to configure
    SDKs (i.e. TracerProvider, MeterProvider, Processors...)
    to reduce the amount of manual configuration required.
    """

    _instance = None
    _is_instrumented = False

    def __new__(cls, *args, **kwargs):

        if cls._instance is None:
            cls._instance = object.__new__(cls, *args, **kwargs)

        return cls._instance

    @abstractmethod
    def _configure(self, **kwargs):
        """Configure the SDK"""

    def configure(self, **kwargs):
        """Configure the SDK"""
        self._configure(**kwargs)


__all__ = ["BaseConfigurator"]
