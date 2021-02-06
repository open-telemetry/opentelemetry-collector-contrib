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

"""Internal utilities."""


class RpcInfo:
    def __init__(
        self,
        full_method=None,
        metadata=None,
        timeout=None,
        request=None,
        response=None,
        error=None,
    ):
        self.full_method = full_method
        self.metadata = metadata
        self.timeout = timeout
        self.request = request
        self.response = response
        self.error = error
