from ..utils.formats import flatten_dict


BLACKLIST_ENDPOINT = ['kms', 'sts']
BLACKLIST_ENDPOINT_TAGS = {
    's3': ['params.Body'],
}


def truncate_arg_value(value, max_len=1024):
    """Truncate values which are bytes and greater than `max_len`.
    Useful for parameters like 'Body' in `put_object` operations.
    """
    if isinstance(value, bytes) and len(value) > max_len:
        return b'...'

    return value


def add_span_arg_tags(span, endpoint_name, args, args_names, args_traced):
    if endpoint_name not in BLACKLIST_ENDPOINT:
        blacklisted = BLACKLIST_ENDPOINT_TAGS.get(endpoint_name, [])
        tags = dict(
            (name, value)
            for (name, value) in zip(args_names, args)
            if name in args_traced
        )
        tags = flatten_dict(tags)
        tags = {
            k: truncate_arg_value(v)
            for k, v in tags.items()
            if k not in blacklisted
        }
        span.set_tags(tags)


REGION = 'aws.region'
AGENT = 'aws.agent'
OPERATION = 'aws.operation'
