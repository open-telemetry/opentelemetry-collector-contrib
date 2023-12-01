#!/bin/bash
set -eux

# Start the Redis containers
docker-compose up -d --remove-orphans

# Function to check if Redis is ready
is_redis_ready() {
    container_id="$1"
    docker exec "$container_id" redis-cli ping > /dev/null 2>&1
}

# Check all Redis containers for readiness
for id in $(docker ps --filter "ancestor=redis" --filter="name=integration-redis-node*" --format "{{.ID}}"); do
    echo "Waiting for Redis container $id to be ready..."
    until is_redis_ready "$id"; do
        sleep 1
    done
done

echo "All Redis instances are ready."
sleep 25s # 10s interval configured, so giving it 2.5*interval to get something
docker logs "$(docker ps --filter "name=integration-redis-otel-collector" --format "{{.ID}}")" 2>&1 | grep "redis.slave.replication.offset" || exit 1

 
