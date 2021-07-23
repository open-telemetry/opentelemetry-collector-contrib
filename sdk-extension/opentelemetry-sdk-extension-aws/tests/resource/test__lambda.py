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

from opentelemetry.sdk.extension.aws.resource._lambda import (
    AwsLambdaResourceDetector,
)
from opentelemetry.semconv.resource import (
    CloudPlatformValues,
    CloudProviderValues,
    ResourceAttributes,
)

MockLambdaResourceAttributes = {
    ResourceAttributes.CLOUD_PROVIDER: CloudProviderValues.AWS.value,
    ResourceAttributes.CLOUD_PLATFORM: CloudPlatformValues.AWS_LAMBDA.value,
    ResourceAttributes.CLOUD_REGION: "mock-west-2",
    ResourceAttributes.FAAS_NAME: "mock-lambda-name",
    ResourceAttributes.FAAS_VERSION: "mock-version-42",
    ResourceAttributes.FAAS_INSTANCE: "mock-log-stream",
    ResourceAttributes.FAAS_MAX_MEMORY: 128,
}


class AwsLambdaResourceDetectorTest(unittest.TestCase):
    @patch.dict(
        "os.environ",
        {
            "AWS_REGION": MockLambdaResourceAttributes[
                ResourceAttributes.CLOUD_REGION
            ],
            "AWS_LAMBDA_FUNCTION_NAME": MockLambdaResourceAttributes[
                ResourceAttributes.FAAS_NAME
            ],
            "AWS_LAMBDA_FUNCTION_VERSION": MockLambdaResourceAttributes[
                ResourceAttributes.FAAS_VERSION
            ],
            "AWS_LAMBDA_LOG_STREAM_NAME": MockLambdaResourceAttributes[
                ResourceAttributes.FAAS_INSTANCE
            ],
            "AWS_LAMBDA_FUNCTION_MEMORY_SIZE": f"{MockLambdaResourceAttributes[ResourceAttributes.FAAS_MAX_MEMORY]}",
        },
        clear=True,
    )
    def test_simple_create(self):
        actual = AwsLambdaResourceDetector().detect()
        self.assertDictEqual(
            actual.attributes.copy(), OrderedDict(MockLambdaResourceAttributes)
        )
