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
import pkg_resources

# IMPORTANT: Only the wsgi module needs this because it is always the first
# package that uses the `{rootdir}/*/tests/` path and gets installed by
# `eachdist.py` and according to `eachdist.ini`.

# Naming the tests module as a namespace package ensures that
# relative imports will resolve properly for subsequent test packages,
# as it enables searching for a composite of multiple test modules.
pkg_resources.declare_namespace(__name__)
