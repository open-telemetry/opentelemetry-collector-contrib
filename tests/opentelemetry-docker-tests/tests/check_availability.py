# Copyright The OpenTelemetry Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
import logging
import os
import time

import mysql.connector
import psycopg2
import pymongo
import redis

MONGODB_COLLECTION_NAME = "test"
MONGODB_DB_NAME = os.getenv("MONGODB_DB_NAME", "opentelemetry-tests")
MONGODB_HOST = os.getenv("MONGODB_HOST", "localhost")
MONGODB_PORT = int(os.getenv("MONGODB_PORT", "27017"))
MYSQL_DB_NAME = os.getenv("MYSQL_DB_NAME ", "opentelemetry-tests")
MYSQL_HOST = os.getenv("MYSQL_HOST ", "localhost")
MYSQL_PORT = int(os.getenv("MYSQL_PORT ", "3306"))
MYSQL_USER = os.getenv("MYSQL_USER ", "testuser")
MYSQL_PASSWORD = os.getenv("MYSQL_PASSWORD ", "testpassword")
POSTGRES_DB_NAME = os.getenv("POSTGRESQL_DB_NAME", "opentelemetry-tests")
POSTGRES_HOST = os.getenv("POSTGRESQL_HOST", "localhost")
POSTGRES_PASSWORD = os.getenv("POSTGRESQL_HOST", "testpassword")
POSTGRES_PORT = int(os.getenv("POSTGRESQL_PORT", "5432"))
POSTGRES_USER = os.getenv("POSTGRESQL_HOST", "testuser")
REDIS_HOST = os.getenv("REDIS_HOST", "localhost")
REDIS_PORT = int(os.getenv("REDIS_PORT ", "6379"))
RETRY_COUNT = 5
RETRY_INTERVAL = 5  # Seconds

logger = logging.getLogger(__name__)


def retryable(func):
    def wrapper():
        # Try to connect to DB
        for i in range(RETRY_COUNT):
            try:
                func()
                return
            except Exception as ex:  # pylint: disable=broad-except
                logger.error(
                    "waiting for %s, retry %d/%d [%s]",
                    func.__name__,
                    i + 1,
                    RETRY_COUNT,
                    ex,
                )
            time.sleep(RETRY_INTERVAL)
        raise Exception("waiting for {} failed".format(func.__name__))

    return wrapper


@retryable
def check_pymongo_connection():
    client = pymongo.MongoClient(
        MONGODB_HOST, MONGODB_PORT, serverSelectionTimeoutMS=2000
    )
    db = client[MONGODB_DB_NAME]
    collection = db[MONGODB_COLLECTION_NAME]
    collection.find_one()
    client.close()


@retryable
def check_mysql_connection():
    connection = mysql.connector.connect(
        user=MYSQL_USER,
        password=MYSQL_PASSWORD,
        host=MYSQL_HOST,
        port=MYSQL_PORT,
        database=MYSQL_DB_NAME,
    )
    connection.close()


@retryable
def check_postgres_connection():
    connection = psycopg2.connect(
        dbname=POSTGRES_DB_NAME,
        user=POSTGRES_USER,
        password=POSTGRES_PASSWORD,
        host=POSTGRES_HOST,
        port=POSTGRES_PORT,
    )
    connection.close()


@retryable
def check_redis_connection():
    connection = redis.Redis(host=REDIS_HOST, port=REDIS_PORT)
    connection.hgetall("*")


def check_docker_services_availability():
    # Check if Docker services accept connections
    check_pymongo_connection()
    check_mysql_connection()
    check_postgres_connection()
    check_redis_connection()


check_docker_services_availability()
