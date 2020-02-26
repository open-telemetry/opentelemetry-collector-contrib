from collections import namedtuple

TAG_NAMES = [
    'RESOURCE_NAME',
    'SAMPLING_PRIORITY',
    'SERVICE_NAME',
    'SPAN_TYPE',
    'TARGET_HOST',
    'TARGET_PORT',
]

TagNames = namedtuple('TagNames', TAG_NAMES)

Tags = TagNames(
    RESOURCE_NAME='resource.name',
    SAMPLING_PRIORITY='sampling.priority',
    SERVICE_NAME='service.name',
    TARGET_HOST='out.host',
    TARGET_PORT='out.port',
    SPAN_TYPE='span.type',
)
