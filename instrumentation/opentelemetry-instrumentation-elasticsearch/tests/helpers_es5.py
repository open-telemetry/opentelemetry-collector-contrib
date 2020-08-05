from elasticsearch_dsl import (  # pylint: disable=no-name-in-module
    DocType,
    Keyword,
    Text,
)


class Article(DocType):
    title = Text(analyzer="snowball", fields={"raw": Keyword()})
    body = Text(analyzer="snowball")

    class Meta:
        index = "test-index"


dsl_create_statement = {
    "mappings": {
        "article": {
            "properties": {
                "title": {
                    "analyzer": "snowball",
                    "fields": {"raw": {"type": "keyword"}},
                    "type": "text",
                },
                "body": {"analyzer": "snowball", "type": "text"},
            }
        }
    },
}
dsl_index_result = (1, {}, '{"created": true}')
dsl_index_span_name = "Elasticsearch/test-index/article/2"
dsl_index_url = "/test-index/article/2"
dsl_search_method = "GET"
