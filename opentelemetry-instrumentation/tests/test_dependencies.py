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

# pylint: disable=protected-access

import pkg_resources
import pytest

from opentelemetry.instrumentation.dependencies import (
    DependencyConflict,
    get_dependency_conflicts,
    get_dist_dependency_conflicts,
)
from opentelemetry.test.test_base import TestBase


class TestDependencyConflicts(TestBase):
    def test_get_dependency_conflicts_empty(self):
        self.assertIsNone(get_dependency_conflicts([]))

    def test_get_dependency_conflicts_no_conflict(self):
        self.assertIsNone(get_dependency_conflicts(["pytest"]))

    def test_get_dependency_conflicts_not_installed(self):
        conflict = get_dependency_conflicts(["this-package-does-not-exist"])
        self.assertTrue(conflict is not None)
        self.assertTrue(isinstance(conflict, DependencyConflict))
        self.assertEqual(
            str(conflict),
            'DependencyConflict: requested: "this-package-does-not-exist" but found: "None"',
        )

    def test_get_dependency_conflicts_mismatched_version(self):
        conflict = get_dependency_conflicts(["pytest == 5000"])
        self.assertTrue(conflict is not None)
        self.assertTrue(isinstance(conflict, DependencyConflict))
        self.assertEqual(
            str(conflict),
            f'DependencyConflict: requested: "pytest == 5000" but found: "pytest {pytest.__version__}"',
        )

    def test_get_dist_dependency_conflicts(self):
        def mock_requires(extras=()):
            if "instruments" in extras:
                return [
                    pkg_resources.Requirement(
                        'test-pkg ~= 1.0; extra == "instruments"'
                    )
                ]
            return []

        dist = pkg_resources.Distribution(
            project_name="test-instrumentation", version="1.0"
        )
        dist.requires = mock_requires

        conflict = get_dist_dependency_conflicts(dist)
        self.assertTrue(conflict is not None)
        self.assertTrue(isinstance(conflict, DependencyConflict))
        self.assertEqual(
            str(conflict),
            'DependencyConflict: requested: "test-pkg~=1.0" but found: "None"',
        )
