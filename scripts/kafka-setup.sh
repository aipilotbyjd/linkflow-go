#!/bin/bash

# Kafka Topics Setup Script for LinkFlow

set -e

# Default Kafka connection settings
KAFKA_BOOTSTRAP_SERVER=${KAFKA_BROKERS:-localhost:9092}
KAFKA_CONTAINER=${KAFKA_CONTAINER:-linkflow-go-kafka-1}

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

print_status() {
    echo -e "${GREEN}[KAFKA]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

# Check if Kafka is running
check_kafka() {
    print_status "Checking Kafka connectivity..."
    
    # Try to connect to Kafka using docker exec
    if docker exec -it $KAFKA_CONTAINER kafka-broker-api-versions --bootstrap-server $KAFKA_BOOTSTRAP_SERVER &>/dev/null; then
        print_status "Kafka is running and accessible ✓"
        return 0
    else
        print_error "Cannot connect to Kafka at $KAFKA_BOOTSTRAP_SERVER"
        print_warning "Make sure Kafka is running: docker-compose up -d kafka"
        exit 1
    fi
}

# Create a topic
create_topic() {
    local topic_name=$1
    local partitions=${2:-10}
    local replication=${3:-1}
    local config=${4:-""}
    
    print_status "Creating topic: $topic_name (partitions=$partitions, replication=$replication)"
    
    # Base command
    local cmd="kafka-topics --create --if-not-exists --bootstrap-server $KAFKA_BOOTSTRAP_SERVER --topic $topic_name --partitions $partitions --replication-factor $replication"
    
    # Add config if provided
    if [ -n "$config" ]; then
        cmd="$cmd --config $config"
    fi
    
    # Execute command
    if docker exec -it $KAFKA_CONTAINER $cmd 2>&1 | grep -q "already exists"; then
        print_warning "Topic '$topic_name' already exists"
    else
        docker exec -it $KAFKA_CONTAINER $cmd
        print_status "Topic '$topic_name' created successfully ✓"
    fi
}

# List all topics
list_topics() {
    print_status "Listing all Kafka topics:"
    docker exec -it $KAFKA_CONTAINER kafka-topics --list --bootstrap-server $KAFKA_BOOTSTRAP_SERVER
}

# Describe a topic
describe_topic() {
    local topic_name=$1
    print_status "Describing topic: $topic_name"
    docker exec -it $KAFKA_CONTAINER kafka-topics --describe --bootstrap-server $KAFKA_BOOTSTRAP_SERVER --topic $topic_name
}

# Delete a topic
delete_topic() {
    local topic_name=$1
    print_warning "Deleting topic: $topic_name"
    docker exec -it $KAFKA_CONTAINER kafka-topics --delete --bootstrap-server $KAFKA_BOOTSTRAP_SERVER --topic $topic_name
    print_status "Topic '$topic_name' deleted"
}

# Create all required topics
create_all_topics() {
    print_status "Creating all LinkFlow Kafka topics..."
    
    # Workflow events topic
    create_topic "workflow.events" 10 1 "retention.ms=604800000,compression.type=snappy"
    
    # Execution events topic
    create_topic "execution.events" 20 1 "retention.ms=259200000,compression.type=snappy"
    
    # Audit log topic
    create_topic "audit.log" 5 1 "retention.ms=2592000000,compression.type=gzip"
    
    # Analytics events topic
    create_topic "analytics.events" 15 1 "retention.ms=86400000,compression.type=snappy"
    
    # Notification events topic
    create_topic "notification.events" 10 1 "retention.ms=172800000"
    
    # Dead letter queue topic
    create_topic "dlq.events" 5 1 "retention.ms=1209600000,compression.type=gzip"
    
    # Schedule triggers topic
    create_topic "schedule.triggers" 5 1 "retention.ms=86400000"
    
    # Webhook events topic
    create_topic "webhook.events" 10 1 "retention.ms=259200000"
    
    # State change events topic
    create_topic "state.changes" 15 1 "retention.ms=432000000,compression.type=snappy"
    
    # Metrics topic
    create_topic "metrics.events" 20 1 "retention.ms=86400000,compression.type=snappy"
    
    print_status "All topics created successfully ✓"
}

# Test producer
test_producer() {
    local topic_name=${1:-"workflow.events"}
    local message=${2:-"Test message from LinkFlow"}
    
    print_status "Sending test message to topic: $topic_name"
    echo "$message" | docker exec -i $KAFKA_CONTAINER kafka-console-producer --broker-list $KAFKA_BOOTSTRAP_SERVER --topic $topic_name
    print_status "Message sent successfully ✓"
}

# Test consumer
test_consumer() {
    local topic_name=${1:-"workflow.events"}
    
    print_status "Starting consumer for topic: $topic_name (Press Ctrl+C to stop)"
    docker exec -it $KAFKA_CONTAINER kafka-console-consumer --bootstrap-server $KAFKA_BOOTSTRAP_SERVER --topic $topic_name --from-beginning --max-messages 10
}

# Get topic stats
get_topic_stats() {
    print_status "Getting Kafka topic statistics..."
    
    # List topics with their partition counts
    docker exec -it $KAFKA_CONTAINER kafka-topics --list --bootstrap-server $KAFKA_BOOTSTRAP_SERVER | while read topic; do
        if [ -n "$topic" ]; then
            partitions=$(docker exec -it $KAFKA_CONTAINER kafka-topics --describe --bootstrap-server $KAFKA_BOOTSTRAP_SERVER --topic $topic | grep -c "Partition:")
            echo "  • $topic: $partitions partitions"
        fi
    done
}

# Reset consumer group offset
reset_consumer_group() {
    local group_id=$1
    local topic=$2
    local reset_to=${3:-"earliest"}  # earliest, latest, or specific offset
    
    print_status "Resetting consumer group '$group_id' for topic '$topic' to '$reset_to'"
    docker exec -it $KAFKA_CONTAINER kafka-consumer-groups --bootstrap-server $KAFKA_BOOTSTRAP_SERVER \
        --group $group_id --topic $topic --reset-offsets --to-$reset_to --execute
}

# Show consumer group lag
show_consumer_lag() {
    local group_id=${1:-"linkflow-group"}
    
    print_status "Showing lag for consumer group: $group_id"
    docker exec -it $KAFKA_CONTAINER kafka-consumer-groups --bootstrap-server $KAFKA_BOOTSTRAP_SERVER \
        --group $group_id --describe
}

# Print usage
usage() {
    echo "Usage: $0 [COMMAND] [OPTIONS]"
    echo ""
    echo "Commands:"
    echo "  setup           Create all LinkFlow topics"
    echo "  list            List all topics"
    echo "  describe TOPIC  Describe a specific topic"
    echo "  create TOPIC    Create a single topic"
    echo "  delete TOPIC    Delete a topic"
    echo "  test-producer   Send a test message"
    echo "  test-consumer   Start a test consumer"
    echo "  stats           Show topic statistics"
    echo "  lag [GROUP]     Show consumer group lag"
    echo "  reset GROUP TOPIC  Reset consumer group offset"
    echo ""
    echo "Examples:"
    echo "  $0 setup                              # Create all topics"
    echo "  $0 create my-topic 10 1               # Create topic with 10 partitions"
    echo "  $0 describe workflow.events           # Describe workflow.events topic"
    echo "  $0 test-producer workflow.events      # Send test message"
    echo "  $0 lag linkflow-group                 # Show consumer lag"
    echo ""
}

# Main execution
main() {
    case "$1" in
        setup)
            check_kafka
            create_all_topics
            list_topics
            ;;
        list)
            check_kafka
            list_topics
            ;;
        describe)
            check_kafka
            if [ -z "$2" ]; then
                print_error "Please specify a topic name"
                exit 1
            fi
            describe_topic "$2"
            ;;
        create)
            check_kafka
            if [ -z "$2" ]; then
                print_error "Please specify a topic name"
                exit 1
            fi
            create_topic "$2" "${3:-10}" "${4:-1}" "$5"
            ;;
        delete)
            check_kafka
            if [ -z "$2" ]; then
                print_error "Please specify a topic name"
                exit 1
            fi
            delete_topic "$2"
            ;;
        test-producer)
            check_kafka
            test_producer "$2" "$3"
            ;;
        test-consumer)
            check_kafka
            test_consumer "$2"
            ;;
        stats)
            check_kafka
            get_topic_stats
            ;;
        lag)
            check_kafka
            show_consumer_lag "$2"
            ;;
        reset)
            check_kafka
            if [ -z "$2" ] || [ -z "$3" ]; then
                print_error "Please specify consumer group and topic"
                exit 1
            fi
            reset_consumer_group "$2" "$3" "$4"
            ;;
        *)
            usage
            ;;
    esac
}

# Run main function
main "$@"
