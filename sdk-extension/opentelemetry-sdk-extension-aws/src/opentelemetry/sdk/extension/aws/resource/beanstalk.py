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

import json
import logging
import os

from opentelemetry.sdk.resources import Resource, ResourceDetector
from opentelemetry.semconv.resource import (
    CloudPlatformValues,
    CloudProviderValues,
    ResourceAttributes,
)

logger = logging.getLogger(__name__)


class AwsBeanstalkResourceDetector(ResourceDetector):
    """Detects attribute values only available when the app is running on AWS
    Elastic Beanstalk and returns them in a Resource.

    NOTE: Requires enabling X-Ray on Beanstalk Environment. See more here: https://docs.aws.amazon.com/xray/latest/devguide/xray-services-beanstalk.html
    """

    def detect(self) -> "Resource":
        if os.name == "nt":
            conf_file_path = (
                "C:\\Program Files\\Amazon\\XRay\\environment.conf"
            )
        else:
            conf_file_path = "/var/elasticbeanstalk/xray/environment.conf"

        try:
            with open(conf_file_path) as conf_file:
                parsed_data = json.load(conf_file)

            return Resource(
                {
                    ResourceAttributes.CLOUD_PROVIDER: CloudProviderValues.AWS.value,
                    ResourceAttributes.CLOUD_PLATFORM: CloudPlatformValues.AWS_ELASTIC_BEANSTALK.value,
                    ResourceAttributes.SERVICE_NAME: CloudPlatformValues.AWS_ELASTIC_BEANSTALK.value,
                    ResourceAttributes.SERVICE_INSTANCE_ID: parsed_data[
                        "deployment_id"
                    ],
                    ResourceAttributes.SERVICE_NAMESPACE: parsed_data[
                        "environment_name"
                    ],
                    ResourceAttributes.SERVICE_VERSION: parsed_data[
                        "version_label"
                    ],
                }
            )
        # pylint: disable=broad-except
        except Exception as exception:
            if self.raise_on_error:
                raise exception

            logger.warning("%s failed: %s", self.__class__.__name__, exception)
            return Resource.get_empty()
