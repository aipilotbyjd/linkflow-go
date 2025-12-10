#!/bin/bash

# LinkFlow Database Backup Script
# Performs automated backups with retention and S3 upload

set -e

# Configuration
BACKUP_DIR="${BACKUP_DIR:-/backups}"
RETENTION_DAYS="${RETENTION_DAYS:-30}"
S3_BUCKET="${S3_BUCKET:-linkflow-backups}"
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_NAME="${DB_NAME:-linkflow}"
DB_USER="${DB_USER:-linkflow}"
REDIS_HOST="${REDIS_HOST:-localhost}"
REDIS_PORT="${REDIS_PORT:-6379}"

# Create backup directory with timestamp
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
DATE_DIR=$(date +%Y-%m-%d)
BACKUP_PATH="$BACKUP_DIR/$DATE_DIR"
mkdir -p "$BACKUP_PATH"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $(date '+%Y-%m-%d %H:%M:%S') - $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $(date '+%Y-%m-%d %H:%M:%S') - $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $(date '+%Y-%m-%d %H:%M:%S') - $1"
}

# Backup PostgreSQL
backup_postgres() {
    log_info "Starting PostgreSQL backup..."
    
    # Set password
    export PGPASSWORD="${DB_PASSWORD:-password}"
    
    # Perform backup
    POSTGRES_BACKUP="$BACKUP_PATH/postgres_${TIMESTAMP}.sql.gz"
    
    pg_dump \
        -h "$DB_HOST" \
        -p "$DB_PORT" \
        -U "$DB_USER" \
        -d "$DB_NAME" \
        --verbose \
        --no-owner \
        --no-acl \
        --if-exists \
        --clean \
        --create \
        | gzip -9 > "$POSTGRES_BACKUP"
    
    if [ $? -eq 0 ]; then
        log_info "PostgreSQL backup completed: $POSTGRES_BACKUP"
        echo "$POSTGRES_BACKUP"
    else
        log_error "PostgreSQL backup failed"
        return 1
    fi
}

# Backup Redis
backup_redis() {
    log_info "Starting Redis backup..."
    
    REDIS_BACKUP="$BACKUP_PATH/redis_${TIMESTAMP}.rdb"
    
    # Create Redis backup
    redis-cli -h "$REDIS_HOST" -p "$REDIS_PORT" --rdb "$REDIS_BACKUP"
    
    if [ $? -eq 0 ]; then
        # Compress Redis backup
        gzip -9 "$REDIS_BACKUP"
        REDIS_BACKUP="${REDIS_BACKUP}.gz"
        log_info "Redis backup completed: $REDIS_BACKUP"
        echo "$REDIS_BACKUP"
    else
        log_warning "Redis backup failed (non-critical)"
    fi
}

# Upload to S3
upload_to_s3() {
    local file=$1
    
    if [ -z "$AWS_ACCESS_KEY_ID" ] || [ -z "$AWS_SECRET_ACCESS_KEY" ]; then
        log_warning "AWS credentials not configured, skipping S3 upload"
        return 0
    fi
    
    log_info "Uploading to S3: $file"
    
    # Extract filename
    filename=$(basename "$file")
    
    # Upload to S3 with date-based organization
    aws s3 cp "$file" "s3://$S3_BUCKET/$DATE_DIR/$filename" \
        --storage-class STANDARD_IA \
        --metadata "backup-date=$TIMESTAMP"
    
    if [ $? -eq 0 ]; then
        log_info "Successfully uploaded to S3: s3://$S3_BUCKET/$DATE_DIR/$filename"
    else
        log_error "Failed to upload to S3"
        return 1
    fi
}

# Cleanup old backups
cleanup_old_backups() {
    log_info "Cleaning up old backups (retention: $RETENTION_DAYS days)..."
    
    # Local cleanup
    find "$BACKUP_DIR" -type d -mtime +$RETENTION_DAYS -exec rm -rf {} \; 2>/dev/null || true
    
    # S3 cleanup (if configured)
    if [ ! -z "$AWS_ACCESS_KEY_ID" ]; then
        # Calculate cutoff date
        CUTOFF_DATE=$(date -d "$RETENTION_DAYS days ago" +%Y-%m-%d)
        
        # List and delete old S3 objects
        aws s3api list-objects-v2 \
            --bucket "$S3_BUCKET" \
            --query "Contents[?LastModified<'$CUTOFF_DATE'].Key" \
            --output text | \
        while read -r key; do
            if [ ! -z "$key" ]; then
                aws s3 rm "s3://$S3_BUCKET/$key"
                log_info "Deleted old S3 backup: $key"
            fi
        done
    fi
    
    log_info "Cleanup completed"
}

# Verify backup integrity
verify_backup() {
    local file=$1
    
    log_info "Verifying backup integrity: $file"
    
    # Test gzip integrity
    gzip -t "$file" 2>/dev/null
    
    if [ $? -eq 0 ]; then
        log_info "Backup verification passed"
        return 0
    else
        log_error "Backup verification failed"
        return 1
    fi
}

# Send notification (optional)
send_notification() {
    local status=$1
    local message=$2
    
    # Slack webhook (if configured)
    if [ ! -z "$SLACK_WEBHOOK_URL" ]; then
        curl -X POST "$SLACK_WEBHOOK_URL" \
            -H 'Content-Type: application/json' \
            -d "{\"text\":\"Backup $status: $message\"}" \
            2>/dev/null || true
    fi
    
    # Email notification (if configured)
    if [ ! -z "$EMAIL_TO" ] && command -v mail &> /dev/null; then
        echo "$message" | mail -s "LinkFlow Backup $status" "$EMAIL_TO"
    fi
}

# Create backup metadata
create_metadata() {
    local postgres_file=$1
    local redis_file=$2
    
    METADATA_FILE="$BACKUP_PATH/metadata_${TIMESTAMP}.json"
    
    # Get file sizes
    postgres_size=$(stat -c%s "$postgres_file" 2>/dev/null || stat -f%z "$postgres_file" 2>/dev/null)
    redis_size=$(stat -c%s "$redis_file" 2>/dev/null || stat -f%z "$redis_file" 2>/dev/null || echo 0)
    
    # Get database statistics
    db_tables=$(PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public';" 2>/dev/null || echo 0)
    
    cat > "$METADATA_FILE" <<EOF
{
    "timestamp": "$TIMESTAMP",
    "date": "$DATE_DIR",
    "backups": {
        "postgres": {
            "file": "$(basename $postgres_file)",
            "size": $postgres_size,
            "tables": $db_tables
        },
        "redis": {
            "file": "$(basename $redis_file)",
            "size": $redis_size
        }
    },
    "retention_days": $RETENTION_DAYS,
    "host": "$(hostname)",
    "version": "1.0.0"
}
EOF
    
    log_info "Created backup metadata: $METADATA_FILE"
}

# Main backup process
main() {
    log_info "Starting LinkFlow backup process..."
    log_info "Backup directory: $BACKUP_PATH"
    
    # Initialize status
    BACKUP_STATUS="SUCCESS"
    BACKUP_MESSAGE="Backup completed successfully"
    
    # Perform PostgreSQL backup
    POSTGRES_BACKUP=$(backup_postgres)
    if [ $? -ne 0 ]; then
        BACKUP_STATUS="FAILED"
        BACKUP_MESSAGE="PostgreSQL backup failed"
        log_error "$BACKUP_MESSAGE"
        send_notification "$BACKUP_STATUS" "$BACKUP_MESSAGE"
        exit 1
    fi
    
    # Verify PostgreSQL backup
    verify_backup "$POSTGRES_BACKUP"
    if [ $? -ne 0 ]; then
        BACKUP_STATUS="FAILED"
        BACKUP_MESSAGE="PostgreSQL backup verification failed"
        log_error "$BACKUP_MESSAGE"
        send_notification "$BACKUP_STATUS" "$BACKUP_MESSAGE"
        exit 1
    fi
    
    # Perform Redis backup
    REDIS_BACKUP=$(backup_redis)
    
    # Create metadata
    create_metadata "$POSTGRES_BACKUP" "$REDIS_BACKUP"
    
    # Upload to S3
    if [ ! -z "$S3_BUCKET" ]; then
        upload_to_s3 "$POSTGRES_BACKUP"
        [ ! -z "$REDIS_BACKUP" ] && upload_to_s3 "$REDIS_BACKUP"
        upload_to_s3 "$METADATA_FILE"
    fi
    
    # Cleanup old backups
    cleanup_old_backups
    
    # Calculate backup size
    TOTAL_SIZE=$(du -sh "$BACKUP_PATH" | cut -f1)
    
    # Success message
    BACKUP_MESSAGE="Backup completed successfully. Total size: $TOTAL_SIZE"
    log_info "$BACKUP_MESSAGE"
    
    # Send notification
    send_notification "$BACKUP_STATUS" "$BACKUP_MESSAGE"
    
    log_info "Backup process completed"
}

# Run main function
main
