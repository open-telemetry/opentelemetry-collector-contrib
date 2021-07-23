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

from opentelemetry.sdk.extension.aws.resource.beanstalk import (
    AwsBeanstalkResourceDetector,
)
from opentelemetry.semconv.resource import (
    CloudPlatformValues,
    CloudProviderValues,
    ResourceAttributes,
)

MockBeanstalkResourceAttributes = {
    ResourceAttributes.CLOUD_PROVIDER: CloudProviderValues.AWS.value,
    ResourceAttributes.CLOUD_PLATFORM: CloudPlatformValues.AWS_ELASTIC_BEANSTALK.value,
    ResourceAttributes.SERVICE_NAME: CloudPlatformValues.AWS_ELASTIC_BEANSTALK.value,
    ResourceAttributes.SERVICE_INSTANCE_ID: "42",
    ResourceAttributes.SERVICE_NAMESPACE: "mock-env-name",
    ResourceAttributes.SERVICE_VERSION: "app-zip-label",
}


class AwsBeanstalkResourceDetectorTest(unittest.TestCase):
    @patch(
        "builtins.open",
        new_callable=mock_open,
        read_data=f'{{"deployment_id":"{MockBeanstalkResourceAttributes[ResourceAttributes.SERVICE_INSTANCE_ID]}","environment_name":"{MockBeanstalkResourceAttributes[ResourceAttributes.SERVICE_NAMESPACE]}","version_label":"{MockBeanstalkResourceAttributes[ResourceAttributes.SERVICE_VERSION]}"}}',
    )
    def test_simple_create(self, mock_open_function):
        actual = AwsBeanstalkResourceDetector().detect()
        self.assertDictEqual(
            actual.attributes.copy(),
            OrderedDict(MockBeanstalkResourceAttributes),
        )
