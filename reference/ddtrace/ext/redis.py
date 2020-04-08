from . import SpanTypes

# defaults
APP = 'redis'
DEFAULT_SERVICE = 'redis'

# [TODO] Deprecated, remove when we remove AppTypes
# type of the spans
TYPE = SpanTypes.REDIS

# net extension
DB = 'out.redis_db'

# standard tags
RAWCMD = 'redis.raw_command'
CMD = 'redis.command'
ARGS_LEN = 'redis.args_length'
PIPELINE_LEN = 'redis.pipeline_length'
PIPELINE_AGE = 'redis.pipeline_age'
