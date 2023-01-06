#!/usr/bin/env sh
#
#   Copyright The OpenTelemetry Authors.
#
#   Licensed under the Apache License, Version 2.0 (the "License");
#   you may not use this file except in compliance with the License.
#   You may obtain a copy of the License at
#
#       http://www.apache.org/licenses/LICENSE-2.0
#
#   Unless required by applicable law or agreed to in writing, software
#   distributed under the License is distributed on an "AS IS" BASIS,
#   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#   See the License for the specific language governing permissions and
#   limitations under the License.
#
# Get a list of components within the repository that have some form of ownership
# ascribed to them.

grep -E '^[A-Za-z0-9/]' .github/CODEOWNERS | \
    awk '{ print $1 }' | \
    sed -E 's%(.+)/$%\1%'
