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

import unittest
from collections import OrderedDict
from unittest.mock import mock_open, patch

from opentelemetry.sdk.extension.aws.resource.ecs import AwsEcsResourceDetector
from opentelemetry.semconv.resource import (
    CloudPlatformValues,
    CloudProviderValues,
    ResourceAttributes,
)

MockEcsResourceAttributes = {
    ResourceAttributes.CLOUD_PROVIDER: CloudProviderValues.AWS.value,
    ResourceAttributes.CLOUD_PLATFORM: CloudPlatformValues.AWS_ECS.value,
    ResourceAttributes.CONTAINER_NAME: "mock-container-name",
    ResourceAttributes.CONTAINER_ID: "a4d00c9dd675d67f866c786181419e1b44832d4696780152e61afd44a3e02856",
}


class AwsEcsResourceDetectorTest(unittest.TestCase):
    @patch.dict(
        "os.environ",
        {"ECS_CONTAINER_METADATA_URI": "mock-uri"},
        clear=True,
    )
    @patch(
        "socket.gethostname",
        return_value=f"{MockEcsResourceAttributes[ResourceAttributes.CONTAINER_NAME]}",
    )
    @patch(
        "builtins.open",
        new_callable=mock_open,
        read_data=f"""14:name=systemd:/docker/{MockEcsResourceAttributes[ResourceAttributes.CONTAINER_ID]}
13:rdma:/
12:pids:/docker/bogusContainerIdThatShouldNotBeOneSetBecauseTheFirstOneWasPicked
11:hugetlb:/docker/bogusContainerIdThatShouldNotBeOneSetBecauseTheFirstOneWasPicked
10:net_prio:/docker/bogusContainerIdThatShouldNotBeOneSetBecauseTheFirstOneWasPicked
9:perf_event:/docker/bogusContainerIdThatShouldNotBeOneSetBecauseTheFirstOneWasPicked
8:net_cls:/docker/bogusContainerIdThatShouldNotBeOneSetBecauseTheFirstOneWasPicked
7:freezer:/docker/
6:devices:/docker/bogusContainerIdThatShouldNotBeOneSetBecauseTheFirstOneWasPicked
5:memory:/docker/bogusContainerIdThatShouldNotBeOneSetBecauseTheFirstOneWasPicked
4:blkio:/docker/bogusContainerIdThatShouldNotBeOneSetBecauseTheFirstOneWasPicked
3:cpuacct:/docker/bogusContainerIdThatShouldNotBeOneSetBecauseTheFirstOneWasPicked
2:cpu:/docker/bogusContainerIdThatShouldNotBeOneSetBecauseTheFirstOneWasPicked
1:cpuset:/docker/bogusContainerIdThatShouldNotBeOneSetBecauseTheFirstOneWasPicked
""",
    )
    def test_simple_create(self, mock_open_function, mock_socket_gethostname):
        actual = AwsEcsResourceDetector().detect()
        self.assertDictEqual(
            actual.attributes.copy(), OrderedDict(MockEcsResourceAttributes)
        )
