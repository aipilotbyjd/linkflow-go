#!/bin/bash
set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m'

KAFKA_CONTAINER=${KAFKA_CONTAINER:-linkflow-go-kafka-1}
BOOTSTRAP_SERVER=${BOOTSTRAP_SERVER:-localhost:29092}

# Topics configuration
declare -A TOPICS=(
    ["workflow.events"]=10
    ["workflow.triggers"]=5
    ["execution.events"]=20
    ["execution.commands"]=15
    ["execution.results"]=20
    ["webhook.events"]=10
    ["schedule.triggers"]=5
    ["notification.events"]=10
    ["analytics.events"]=15
    ["audit.log"]=5
    ["user.events"]=5
    ["credential.events"]=5
    ["billing.events"]=5
    ["dlq.events"]=5
)

setup_topics() {
    echo -e "${GREEN}Setting up Kafka topics...${NC}"
    
    for topic in "${!TOPICS[@]}"; do
        partitions=${TOPICS[$topic]}
        echo -e "${YELLOW}Creating topic: $topic (partitions: $partitions)${NC}"
        
        docker exec $KAFKA_CONTAINER kafka-topics --create \
            --if-not-exists \
            --bootstrap-server $BOOTSTRAP_SERVER \
            --topic $topic \
            --partitions $partitions \
            --replication-factor 1 \
            2>/dev/null || echo "Topic $topic may already exist"
    done
    
    echo -e "${GREEN}Kafka topics setup complete!${NC}"
}

list_topics() {
    echo -e "${GREEN}Listing Kafka topics...${NC}"
    docker exec $KAFKA_CONTAINER kafka-topics --list --bootstrap-server $BOOTSTRAP_SERVER
}

describe_topic() {
    local topic=$1
    echo -e "${GREEN}Describing topic: $topic${NC}"
    docker exec $KAFKA_CONTAINER kafka-topics --describe --topic $topic --bootstrap-server $BOOTSTRAP_SERVER
}

delete_topic() {
    local topic=$1
    echo -e "${YELLOW}Deleting topic: $topic${NC}"
    docker exec $KAFKA_CONTAINER kafka-topics --delete --topic $topic --bootstrap-server $BOOTSTRAP_SERVER
}

consumer_groups() {
    echo -e "${GREEN}Listing consumer groups...${NC}"
    docker exec $KAFKA_CONTAINER kafka-consumer-groups --list --bootstrap-server $BOOTSTRAP_SERVER
}

consumer_lag() {
    local group=${1:-linkflow-group}
    echo -e "${GREEN}Consumer lag for group: $group${NC}"
    docker exec $KAFKA_CONTAINER kafka-consumer-groups --describe --group $group --bootstrap-server $BOOTSTRAP_SERVER
}

case "${1:-setup}" in
    setup)
        setup_topics
        ;;
    list)
        list_topics
        ;;
    describe)
        describe_topic $2
        ;;
    delete)
        delete_topic $2
        ;;
    groups)
        consumer_groups
        ;;
    lag)
        consumer_lag $2
        ;;
    *)
        echo "Usage: $0 {setup|list|describe <topic>|delete <topic>|groups|lag [group]}"
        exit 1
        ;;
esac
