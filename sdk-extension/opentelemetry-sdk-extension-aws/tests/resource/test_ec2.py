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
from unittest.mock import patch

from opentelemetry.sdk.extension.aws.resource.ec2 import AwsEc2ResourceDetector
from opentelemetry.semconv.resource import (
    CloudPlatformValues,
    CloudProviderValues,
    ResourceAttributes,
)

MockEc2ResourceAttributes = {
    ResourceAttributes.CLOUD_PROVIDER: CloudProviderValues.AWS.value,
    ResourceAttributes.CLOUD_PLATFORM: CloudPlatformValues.AWS_EC2.value,
    ResourceAttributes.CLOUD_ACCOUNT_ID: "123456789012",
    ResourceAttributes.CLOUD_REGION: "mock-west-2",
    ResourceAttributes.CLOUD_AVAILABILITY_ZONE: "mock-west-2a",
    ResourceAttributes.HOST_ID: "i-1234ab56cd7e89f01",
    ResourceAttributes.HOST_TYPE: "t2.micro-mock",
    ResourceAttributes.HOST_NAME: "ip-172-12-34-567.mock-west-2.compute.internal",
}


class AwsEc2ResourceDetectorTest(unittest.TestCase):
    @patch(
        "opentelemetry.sdk.extension.aws.resource.ec2._get_host",
        return_value=MockEc2ResourceAttributes[ResourceAttributes.HOST_NAME],
    )
    @patch(
        "opentelemetry.sdk.extension.aws.resource.ec2._get_identity",
        return_value=f"""{{
  "accountId" : "{MockEc2ResourceAttributes[ResourceAttributes.CLOUD_ACCOUNT_ID]}",
  "architecture" : "x86_64",
  "availabilityZone" : "{MockEc2ResourceAttributes[ResourceAttributes.CLOUD_AVAILABILITY_ZONE]}",
  "billingProducts" : null,
  "devpayProductCodes" : null,
  "marketplaceProductCodes" : null,
  "imageId" : "ami-0957cee1854021123",
  "instanceId" : "{MockEc2ResourceAttributes[ResourceAttributes.HOST_ID]}",
  "instanceType" : "{MockEc2ResourceAttributes[ResourceAttributes.HOST_TYPE]}",
  "kernelId" : null,
  "pendingTime" : "2021-07-13T21:53:41Z",
  "privateIp" : "172.12.34.567",
  "ramdiskId" : null,
  "region" : "{MockEc2ResourceAttributes[ResourceAttributes.CLOUD_REGION]}",
  "version" : "2017-09-30"
}}""",
    )
    @patch(
        "opentelemetry.sdk.extension.aws.resource.ec2._get_token",
        return_value="mock-token",
    )
    def test_simple_create(
        self, mock_get_token, mock_get_identity, mock_get_host
    ):
        actual = AwsEc2ResourceDetector().detect()
        self.assertDictEqual(
            actual.attributes.copy(), OrderedDict(MockEc2ResourceAttributes)
        )
