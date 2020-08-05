from elasticsearch_dsl import (  # pylint: disable=no-name-in-module
    DocType,
    String,
)


class Article(DocType):
    title = String(analyzer="snowball", fields={"raw": String()})
    body = String(analyzer="snowball")

    class Meta:
        index = "test-index"


dsl_create_statement = {
    "mappings": {
        "article": {
            "properties": {
                "title": {
                    "analyzer": "snowball",
                    "fields": {"raw": {"type": "string"}},
                    "type": "string",
                },
                "body": {"analyzer": "snowball", "type": "string"},
            }
        }
    },
    "settings": {"analysis": {}},
}
dsl_index_result = (1, {}, '{"created": true}')
dsl_index_span_name = "Elasticsearch/test-index/article/2"
dsl_index_url = "/test-index/article/2"
dsl_search_method = "GET"
