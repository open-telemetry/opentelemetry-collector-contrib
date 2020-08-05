from elasticsearch_dsl import (  # pylint: disable=unused-import
    Document,
    Keyword,
    Text,
)


class Article(Document):
    title = Text(analyzer="snowball", fields={"raw": Keyword()})
    body = Text(analyzer="snowball")

    class Index:
        name = "test-index"


dsl_create_statement = {
    "mappings": {
        "doc": {
            "properties": {
                "title": {
                    "analyzer": "snowball",
                    "fields": {"raw": {"type": "keyword"}},
                    "type": "text",
                },
                "body": {"analyzer": "snowball", "type": "text"},
            }
        }
    }
}
dsl_index_result = (1, {}, '{"result": "created"}')
dsl_index_span_name = "Elasticsearch/test-index/doc/2"
dsl_index_url = "/test-index/doc/2"
dsl_search_method = "GET"
