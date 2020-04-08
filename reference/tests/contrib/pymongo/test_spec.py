"""
tests for parsing specs.
"""

from bson.son import SON

from ddtrace.contrib.pymongo.parse import parse_spec


def test_empty():
    cmd = parse_spec(SON([]))
    assert cmd is None


def test_create():
    cmd = parse_spec(SON([('create', 'foo')]))
    assert cmd.name == 'create'
    assert cmd.coll == 'foo'
    assert cmd.tags == {}
    assert cmd.metrics == {}


def test_insert():
    spec = SON([
        ('insert', 'bla'),
        ('ordered', True),
        ('documents', ['a', 'b']),
    ])
    cmd = parse_spec(spec)
    assert cmd.name == 'insert'
    assert cmd.coll == 'bla'
    assert cmd.tags == {'mongodb.ordered': True}
    assert cmd.metrics == {'mongodb.documents': 2}


def test_update():
    spec = SON([
        ('update', u'songs'),
        ('ordered', True),
        ('updates', [
            SON([
                ('q', {'artist': 'Neil'}),
                ('u', {'$set': {'artist': 'Shakey'}}),
                ('multi', True),
                ('upsert', False)
            ])
        ])
    ])
    cmd = parse_spec(spec)
    assert cmd.name == 'update'
    assert cmd.coll == 'songs'
    assert cmd.query == {'artist': 'Neil'}
