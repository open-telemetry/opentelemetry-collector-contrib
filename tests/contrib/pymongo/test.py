# stdlib
import time
import unittest

# 3p
import pymongo

# project
from ddtrace import Pin
from ddtrace.constants import ANALYTICS_SAMPLE_RATE_KEY
from ddtrace.ext import mongo as mongox
from ddtrace.contrib.pymongo.client import trace_mongo_client, normalize_filter
from ddtrace.contrib.pymongo.patch import patch, unpatch

# testing
from tests.opentracer.utils import init_tracer
from ..config import MONGO_CONFIG
from ...base import override_config
from ...test_tracer import get_dummy_tracer


def test_normalize_filter():
    # ensure we can properly normalize queries FIXME[matt] move to the agent
    cases = [
        (None, {}),
        (
            {'team': 'leafs'},
            {'team': '?'},
        ),
        (
            {'age': {'$gt': 20}},
            {'age': {'$gt': '?'}},
        ),
        (
            {'age': {'$gt': 20}},
            {'age': {'$gt': '?'}},
        ),
        (
            {'_id': {'$in': [1, 2, 3]}},
            {'_id': {'$in': '?'}},
        ),
        (
            {'_id': {'$nin': [1, 2, 3]}},
            {'_id': {'$nin': '?'}},
        ),

        (
            20,
            {},
        ),
        (
            {
                'status': 'A',
                '$or': [{'age': {'$lt': 30}}, {'type': 1}],
            },
            {
                'status': '?',
                '$or': [{'age': {'$lt': '?'}}, {'type': '?'}],
            },
        ),
    ]
    for i, expected in cases:
        out = normalize_filter(i)
        assert expected == out


class PymongoCore(object):
    """Test suite for pymongo

    Independant of the way it got instrumented.
    TODO: merge to a single class when patching is the only way.
    """

    TEST_SERVICE = 'test-mongo'

    def get_tracer_and_client(service):
        # implement me
        pass

    def test_update(self):
        # ensure we trace deletes
        tracer, client = self.get_tracer_and_client()
        writer = tracer.writer
        db = client['testdb']
        db.drop_collection('songs')
        input_songs = [
            {'name': 'Powderfinger', 'artist': 'Neil'},
            {'name': 'Harvest', 'artist': 'Neil'},
            {'name': 'Suzanne', 'artist': 'Leonard'},
            {'name': 'Partisan', 'artist': 'Leonard'},
        ]
        db.songs.insert_many(input_songs)

        result = db.songs.update_many(
            {'artist': 'Neil'},
            {'$set': {'artist': 'Shakey'}},
        )

        assert result.matched_count == 2
        assert result.modified_count == 2

        # ensure all is traced.
        spans = writer.pop()
        assert spans, spans
        for span in spans:
            # ensure all the of the common metadata is set
            assert span.service == self.TEST_SERVICE
            assert span.span_type == 'mongodb'
            assert span.meta.get('mongodb.collection') == 'songs'
            assert span.meta.get('mongodb.db') == 'testdb'
            assert span.meta.get('out.host')
            assert span.metrics.get('out.port')

        expected_resources = set([
            'drop songs',
            'update songs {"artist": "?"}',
            'insert songs',
        ])

        assert expected_resources == {s.resource for s in spans}

    def test_delete(self):
        # ensure we trace deletes
        tracer, client = self.get_tracer_and_client()
        writer = tracer.writer
        db = client['testdb']
        collection_name = 'here.are.songs'
        db.drop_collection(collection_name)
        input_songs = [
            {'name': 'Powderfinger', 'artist': 'Neil'},
            {'name': 'Harvest', 'artist': 'Neil'},
            {'name': 'Suzanne', 'artist': 'Leonard'},
            {'name': 'Partisan', 'artist': 'Leonard'},
        ]

        songs = db[collection_name]
        songs.insert_many(input_songs)

        # test delete one
        af = {'artist': 'Neil'}
        assert songs.count(af) == 2
        songs.delete_one(af)
        assert songs.count(af) == 1

        # test delete many
        af = {'artist': 'Leonard'}
        assert songs.count(af) == 2
        songs.delete_many(af)
        assert songs.count(af) == 0

        # ensure all is traced.
        spans = writer.pop()
        assert spans, spans
        for span in spans:
            # ensure all the of the common metadata is set
            assert span.service == self.TEST_SERVICE
            assert span.span_type == 'mongodb'
            assert span.meta.get('mongodb.collection') == collection_name
            assert span.meta.get('mongodb.db') == 'testdb'
            assert span.meta.get('out.host')
            assert span.metrics.get('out.port')

        expected_resources = [
            'drop here.are.songs',
            'count here.are.songs',
            'count here.are.songs',
            'count here.are.songs',
            'count here.are.songs',
            'delete here.are.songs {"artist": "?"}',
            'delete here.are.songs {"artist": "?"}',
            'insert here.are.songs',
        ]

        assert sorted(expected_resources) == sorted(s.resource for s in spans)

    def test_insert_find(self):
        tracer, client = self.get_tracer_and_client()
        writer = tracer.writer

        start = time.time()
        db = client.testdb
        db.drop_collection('teams')
        teams = [
            {
                'name': 'Toronto Maple Leafs',
                'established': 1917,
            },
            {
                'name': 'Montreal Canadiens',
                'established': 1910,
            },
            {
                'name': 'New York Rangers',
                'established': 1926,
            }
        ]

        # create some data (exercising both ways of inserting)

        db.teams.insert_one(teams[0])
        db.teams.insert_many(teams[1:])

        # wildcard query (using the [] syntax)
        cursor = db['teams'].find()
        count = 0
        for row in cursor:
            count += 1
        assert count == len(teams)

        # scoped query (using the getattr syntax)
        q = {'name': 'Toronto Maple Leafs'}
        queried = list(db.teams.find(q))
        end = time.time()
        assert len(queried) == 1
        assert queried[0]['name'] == 'Toronto Maple Leafs'
        assert queried[0]['established'] == 1917

        spans = writer.pop()
        for span in spans:
            # ensure all the of the common metadata is set
            assert span.service == self.TEST_SERVICE
            assert span.span_type == 'mongodb'
            assert span.meta.get('mongodb.collection') == 'teams'
            assert span.meta.get('mongodb.db') == 'testdb'
            assert span.meta.get('out.host'), span.pprint()
            assert span.metrics.get('out.port'), span.pprint()
            assert span.start > start
            assert span.duration < end - start

        expected_resources = [
            'drop teams',
            'insert teams',
            'insert teams',
        ]

        # query names should be used in >3.1
        name = 'find' if pymongo.version_tuple >= (3, 1, 0) else 'query'

        expected_resources.extend([
            '{} teams'.format(name),
            '{} teams {{"name": "?"}}'.format(name),
        ])

        assert expected_resources == list(s.resource for s in spans)

        # confirm query tag for find all
        assert spans[-2].get_tag('mongodb.query') is None

        # confirm query tag find with query criteria on name
        assert spans[-1].get_tag('mongodb.query') == '{\'name\': \'?\'}'

    def test_update_ot(self):
        """OpenTracing version of test_update."""
        tracer, client = self.get_tracer_and_client()
        ot_tracer = init_tracer('mongo_svc', tracer)

        writer = tracer.writer
        with ot_tracer.start_active_span('mongo_op'):
            db = client['testdb']
            db.drop_collection('songs')
            input_songs = [
                {'name': 'Powderfinger', 'artist': 'Neil'},
                {'name': 'Harvest', 'artist': 'Neil'},
                {'name': 'Suzanne', 'artist': 'Leonard'},
                {'name': 'Partisan', 'artist': 'Leonard'},
            ]
            db.songs.insert_many(input_songs)
            result = db.songs.update_many(
                {'artist': 'Neil'},
                {'$set': {'artist': 'Shakey'}},
            )

            assert result.matched_count == 2
            assert result.modified_count == 2

        # ensure all is traced.
        spans = writer.pop()
        assert spans, spans
        assert len(spans) == 4

        ot_span = spans[0]
        assert ot_span.parent_id is None
        assert ot_span.name == 'mongo_op'
        assert ot_span.service == 'mongo_svc'

        for span in spans[1:]:
            # ensure the parenting
            assert span.parent_id == ot_span.span_id
            # ensure all the of the common metadata is set
            assert span.service == self.TEST_SERVICE
            assert span.span_type == 'mongodb'
            assert span.meta.get('mongodb.collection') == 'songs'
            assert span.meta.get('mongodb.db') == 'testdb'
            assert span.meta.get('out.host')
            assert span.metrics.get('out.port')

        expected_resources = set([
            'drop songs',
            'update songs {"artist": "?"}',
            'insert songs',
        ])

        assert expected_resources == {s.resource for s in spans[1:]}

    def test_analytics_default(self):
        tracer, client = self.get_tracer_and_client()
        db = client['testdb']
        db.drop_collection('songs')

        spans = tracer.writer.pop()
        assert len(spans) == 1
        assert spans[0].get_metric(ANALYTICS_SAMPLE_RATE_KEY) is None

    def test_analytics_with_rate(self):
        with override_config(
            'pymongo',
            dict(analytics_enabled=True, analytics_sample_rate=0.5)
        ):
            tracer, client = self.get_tracer_and_client()
            db = client['testdb']
            db.drop_collection('songs')

            spans = tracer.writer.pop()
            assert len(spans) == 1
            assert spans[0].get_metric(ANALYTICS_SAMPLE_RATE_KEY) == 0.5

    def test_analytics_without_rate(self):
        with override_config(
            'pymongo',
            dict(analytics_enabled=True)
        ):
            tracer, client = self.get_tracer_and_client()
            db = client['testdb']
            db.drop_collection('songs')

            spans = tracer.writer.pop()
            assert len(spans) == 1
            assert spans[0].get_metric(ANALYTICS_SAMPLE_RATE_KEY) == 1.0


class TestPymongoTraceClient(unittest.TestCase, PymongoCore):
    """Test suite for pymongo with the legacy trace interface"""

    TEST_SERVICE = 'test-mongo-trace-client'

    def get_tracer_and_client(self):
        tracer = get_dummy_tracer()
        original_client = pymongo.MongoClient(port=MONGO_CONFIG['port'])
        client = trace_mongo_client(original_client, tracer, service=self.TEST_SERVICE)
        return tracer, client


class TestPymongoPatchDefault(unittest.TestCase, PymongoCore):
    """Test suite for pymongo with the default patched library"""

    TEST_SERVICE = mongox.SERVICE

    def setUp(self):
        patch()

    def tearDown(self):
        unpatch()

    def get_tracer_and_client(self):
        tracer = get_dummy_tracer()
        client = pymongo.MongoClient(port=MONGO_CONFIG['port'])
        Pin.get_from(client).clone(tracer=tracer).onto(client)
        return tracer, client

    def test_service(self):
        tracer, client = self.get_tracer_and_client()
        writer = tracer.writer
        db = client['testdb']
        db.drop_collection('songs')

        services = writer.pop_services()
        assert services == {}

    def test_host_kwarg(self):
        # simulate what celery and django do when instantiating a new client
        conf = {
            'host': 'localhost',
        }
        client = pymongo.MongoClient(**conf)

        conf = {
            'host': None,
        }
        client = pymongo.MongoClient(**conf)

        assert client


class TestPymongoPatchConfigured(unittest.TestCase, PymongoCore):
    """Test suite for pymongo with a configured patched library"""

    TEST_SERVICE = 'test-mongo-trace-client'

    def setUp(self):
        patch()

    def tearDown(self):
        unpatch()

    def get_tracer_and_client(self):
        tracer = get_dummy_tracer()
        client = pymongo.MongoClient(port=MONGO_CONFIG['port'])
        Pin(service=self.TEST_SERVICE, tracer=tracer).onto(client)
        return tracer, client

    def test_patch_unpatch(self):
        tracer = get_dummy_tracer()
        writer = tracer.writer

        # Test patch idempotence
        patch()
        patch()

        client = pymongo.MongoClient(port=MONGO_CONFIG['port'])
        Pin.get_from(client).clone(tracer=tracer).onto(client)
        client['testdb'].drop_collection('whatever')

        spans = writer.pop()
        assert spans, spans
        assert len(spans) == 1

        # Test unpatch
        unpatch()

        client = pymongo.MongoClient(port=MONGO_CONFIG['port'])
        client['testdb'].drop_collection('whatever')

        spans = writer.pop()
        assert not spans, spans

        # Test patch again
        patch()

        client = pymongo.MongoClient(port=MONGO_CONFIG['port'])
        Pin.get_from(client).clone(tracer=tracer).onto(client)
        client['testdb'].drop_collection('whatever')

        spans = writer.pop()
        assert spans, spans
        assert len(spans) == 1
