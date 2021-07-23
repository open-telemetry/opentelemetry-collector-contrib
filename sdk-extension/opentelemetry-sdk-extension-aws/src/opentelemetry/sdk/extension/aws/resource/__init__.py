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

from opentelemetry.sdk.extension.aws.resource._lambda import (
    AwsLambdaResourceDetector,
)
from opentelemetry.sdk.extension.aws.resource.beanstalk import (
    AwsBeanstalkResourceDetector,
)
from opentelemetry.sdk.extension.aws.resource.ec2 import AwsEc2ResourceDetector
from opentelemetry.sdk.extension.aws.resource.ecs import AwsEcsResourceDetector
from opentelemetry.sdk.extension.aws.resource.eks import AwsEksResourceDetector

__all__ = [
    "AwsBeanstalkResourceDetector",
    "AwsEc2ResourceDetector",
    "AwsEcsResourceDetector",
    "AwsEksResourceDetector",
    "AwsLambdaResourceDetector",
]
