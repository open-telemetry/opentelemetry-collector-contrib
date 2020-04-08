from . import SpanTypes

# [TODO] Deprecated, remove when we remove AppTypes
# the type of the spans
TYPE = SpanTypes.CASSANDRA

# tags
CLUSTER = 'cassandra.cluster'
KEYSPACE = 'cassandra.keyspace'
CONSISTENCY_LEVEL = 'cassandra.consistency_level'
PAGINATED = 'cassandra.paginated'
ROW_COUNT = 'cassandra.row_count'
PAGE_NUMBER = 'cassandra.page_number'
