from opentelemetry.instrumentation.botocore.extensions.types import (
    _AttributeMapT,
    _AwsSdkExtension,
)


class _SqsExtension(_AwsSdkExtension):
    def extract_attributes(self, attributes: _AttributeMapT):
        queue_url = self._call_context.params.get("QueueUrl")
        if queue_url:
            # TODO: update when semantic conventions exist
            attributes["aws.queue_url"] = queue_url
