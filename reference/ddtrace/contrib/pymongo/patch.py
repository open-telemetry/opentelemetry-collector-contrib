import pymongo

from .client import TracedMongoClient

# Original Client class
_MongoClient = pymongo.MongoClient


def patch():
    setattr(pymongo, 'MongoClient', TracedMongoClient)


def unpatch():
    setattr(pymongo, 'MongoClient', _MongoClient)
