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

from opentelemetry.sdk.extension.aws.resource.eks import AwsEksResourceDetector
from opentelemetry.semconv.resource import (
    CloudPlatformValues,
    CloudProviderValues,
    ResourceAttributes,
)

MockEksResourceAttributes = {
    ResourceAttributes.CLOUD_PROVIDER: CloudProviderValues.AWS.value,
    ResourceAttributes.CLOUD_PLATFORM: CloudPlatformValues.AWS_EKS.value,
    ResourceAttributes.K8S_CLUSTER_NAME: "mock-cluster-name",
    ResourceAttributes.CONTAINER_ID: "a4d00c9dd675d67f866c786181419e1b44832d4696780152e61afd44a3e02856",
}


class AwsEksResourceDetectorTest(unittest.TestCase):
    @patch(
        "opentelemetry.sdk.extension.aws.resource.eks._get_k8s_cred_value",
        return_value="MOCK_TOKEN",
    )
    @patch(
        "opentelemetry.sdk.extension.aws.resource.eks._is_eks",
        return_value=True,
    )
    @patch(
        "opentelemetry.sdk.extension.aws.resource.eks._get_cluster_info",
        return_value=f"""{{
  "kind": "ConfigMap",
  "apiVersion": "v1",
  "metadata": {{
    "name": "cluster-info",
    "namespace": "amazon-cloudwatch",
    "selfLink": "/api/v1/namespaces/amazon-cloudwatch/configmaps/cluster-info",
    "uid": "0734438c-48f4-45c3-b06d-b6f16f7f0e1e",
    "resourceVersion": "25911",
    "creationTimestamp": "2021-07-23T18:41:56Z",
    "annotations": {{
      "kubectl.kubernetes.io/last-applied-configuration": "{{\\"apiVersion\\":\\"v1\\",\\"data\\":{{\\"cluster.name\\":\\"{MockEksResourceAttributes[ResourceAttributes.K8S_CLUSTER_NAME]}\\",\\"logs.region\\":\\"us-west-2\\"}},\\"kind\\":\\"ConfigMap\\",\\"metadata\\":{{\\"annotations\\":{{}},\\"name\\":\\"cluster-info\\",\\"namespace\\":\\"amazon-cloudwatch\\"}}}}\\n"
    }}
  }},
  "data": {{
    "cluster.name": "{MockEksResourceAttributes[ResourceAttributes.K8S_CLUSTER_NAME]}",
    "logs.region": "us-west-2"
  }}
}}
""",
    )
    @patch(
        "builtins.open",
        new_callable=mock_open,
        read_data=f"""14:name=systemd:/docker/{MockEksResourceAttributes[ResourceAttributes.CONTAINER_ID]}
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
    def test_simple_create(
        self,
        mock_open_function,
        mock_get_cluster_info,
        mock_is_eks,
        mock_get_k8_cred_value,
    ):
        actual = AwsEksResourceDetector().detect()
        self.assertDictEqual(
            actual.attributes.copy(), OrderedDict(MockEksResourceAttributes)
        )

    @patch(
        "opentelemetry.sdk.extension.aws.resource.eks._get_k8s_cred_value",
        return_value="MOCK_TOKEN",
    )
    @patch(
        "opentelemetry.sdk.extension.aws.resource.eks._is_eks",
        return_value=False,
    )
    def test_if_no_eks_env_var_and_should_raise(
        self, mock_is_eks, mock_get_k8_cred_value
    ):
        with self.assertRaises(RuntimeError):
            AwsEksResourceDetector(raise_on_error=True).detect()
