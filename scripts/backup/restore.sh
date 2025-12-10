#!/bin/bash

# LinkFlow Database Restore Script
# Restores database from backup with verification

set -e

# Configuration
BACKUP_DIR="${BACKUP_DIR:-/backups}"
S3_BUCKET="${S3_BUCKET:-linkflow-backups}"
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_NAME="${DB_NAME:-linkflow}"
DB_USER="${DB_USER:-linkflow}"
REDIS_HOST="${REDIS_HOST:-localhost}"
REDIS_PORT="${REDIS_PORT:-6379}"

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

# Show usage
usage() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  -d DATE       Backup date (YYYY-MM-DD)"
    echo "  -f FILE       Specific backup file to restore"
    echo "  -s            Download from S3"
    echo "  -p            Restore PostgreSQL only"
    echo "  -r            Restore Redis only"
    echo "  -y            Skip confirmation prompt"
    echo "  -h            Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0 -d 2024-01-01          # Restore from specific date"
    echo "  $0 -f backup.sql.gz        # Restore from specific file"
    echo "  $0 -d 2024-01-01 -s        # Download from S3 and restore"
}

# Parse arguments
while getopts "d:f:sprhy" opt; do
    case $opt in
        d) RESTORE_DATE="$OPTARG" ;;
        f) RESTORE_FILE="$OPTARG" ;;
        s) USE_S3=true ;;
        p) POSTGRES_ONLY=true ;;
        r) REDIS_ONLY=true ;;
        y) SKIP_CONFIRM=true ;;
        h) usage; exit 0 ;;
        *) usage; exit 1 ;;
    esac
done

# Find latest backup if date not specified
find_latest_backup() {
    local backup_type=$1
    
    if [ ! -z "$RESTORE_DATE" ]; then
        BACKUP_PATH="$BACKUP_DIR/$RESTORE_DATE"
    else
        # Find most recent backup directory
        BACKUP_PATH=$(find "$BACKUP_DIR" -type d -maxdepth 1 | sort -r | head -n 1)
        RESTORE_DATE=$(basename "$BACKUP_PATH")
    fi
    
    if [ ! -d "$BACKUP_PATH" ]; then
        log_error "Backup directory not found: $BACKUP_PATH"
        return 1
    fi
    
    # Find backup file
    if [ "$backup_type" = "postgres" ]; then
        BACKUP_FILE=$(find "$BACKUP_PATH" -name "postgres_*.sql.gz" | sort -r | head -n 1)
    else
        BACKUP_FILE=$(find "$BACKUP_PATH" -name "redis_*.rdb.gz" | sort -r | head -n 1)
    fi
    
    if [ -z "$BACKUP_FILE" ]; then
        log_error "No $backup_type backup found in $BACKUP_PATH"
        return 1
    fi
    
    echo "$BACKUP_FILE"
}

# Download from S3
download_from_s3() {
    local date=$1
    
    if [ -z "$AWS_ACCESS_KEY_ID" ] || [ -z "$AWS_SECRET_ACCESS_KEY" ]; then
        log_error "AWS credentials not configured"
        return 1
    fi
    
    log_info "Downloading backups from S3 for date: $date"
    
    # Create local directory
    mkdir -p "$BACKUP_DIR/$date"
    
    # Download all files for the date
    aws s3 sync "s3://$S3_BUCKET/$date/" "$BACKUP_DIR/$date/"
    
    if [ $? -eq 0 ]; then
        log_info "Successfully downloaded backups from S3"
    else
        log_error "Failed to download from S3"
        return 1
    fi
}

# Restore PostgreSQL
restore_postgres() {
    local backup_file=$1
    
    log_info "Restoring PostgreSQL from: $backup_file"
    
    # Set password
    export PGPASSWORD="${DB_PASSWORD:-password}"
    
    # Check if database exists
    DB_EXISTS=$(PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -lqt | cut -d \| -f 1 | grep -w "$DB_NAME" | wc -l)
    
    if [ "$DB_EXISTS" -eq 1 ] && [ "$SKIP_CONFIRM" != "true" ]; then
        echo -e "${YELLOW}WARNING: Database '$DB_NAME' exists and will be replaced.${NC}"
        read -p "Are you sure you want to continue? (y/N) " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            log_info "Restore cancelled"
            exit 0
        fi
    fi
    
    # Drop and recreate database
    log_info "Preparing database..."
    PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d postgres <<EOF
DROP DATABASE IF EXISTS $DB_NAME;
CREATE DATABASE $DB_NAME OWNER $DB_USER;
EOF
    
    # Restore from backup
    log_info "Restoring data..."
    gunzip -c "$backup_file" | PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME"
    
    if [ $? -eq 0 ]; then
        log_info "PostgreSQL restore completed successfully"
        
        # Verify restore
        verify_postgres_restore
    else
        log_error "PostgreSQL restore failed"
        return 1
    fi
}

# Restore Redis
restore_redis() {
    local backup_file=$1
    
    log_info "Restoring Redis from: $backup_file"
    
    # Stop Redis server (if local)
    if [ "$REDIS_HOST" = "localhost" ] && command -v systemctl &> /dev/null; then
        log_info "Stopping Redis server..."
        sudo systemctl stop redis || true
    fi
    
    # Extract backup
    TEMP_RDB="/tmp/redis_restore.rdb"
    gunzip -c "$backup_file" > "$TEMP_RDB"
    
    # Copy to Redis data directory
    if [ "$REDIS_HOST" = "localhost" ]; then
        REDIS_DIR="/var/lib/redis"
        sudo cp "$TEMP_RDB" "$REDIS_DIR/dump.rdb"
        sudo chown redis:redis "$REDIS_DIR/dump.rdb"
        
        # Start Redis server
        log_info "Starting Redis server..."
        sudo systemctl start redis
    else
        log_warning "Remote Redis restore not implemented. Please restore manually."
    fi
    
    # Cleanup
    rm -f "$TEMP_RDB"
    
    log_info "Redis restore completed"
}

# Verify PostgreSQL restore
verify_postgres_restore() {
    log_info "Verifying PostgreSQL restore..."
    
    export PGPASSWORD="${DB_PASSWORD:-password}"
    
    # Check table count
    TABLE_COUNT=$(PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema NOT IN ('pg_catalog', 'information_schema');" 2>/dev/null)
    
    # Check row counts for main tables
    WORKFLOW_COUNT=$(PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "SELECT COUNT(*) FROM workflow.workflows;" 2>/dev/null || echo 0)
    USER_COUNT=$(PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "SELECT COUNT(*) FROM auth.users;" 2>/dev/null || echo 0)
    
    log_info "Verification results:"
    log_info "  - Tables restored: $TABLE_COUNT"
    log_info "  - Workflows: $WORKFLOW_COUNT"
    log_info "  - Users: $USER_COUNT"
    
    if [ "$TABLE_COUNT" -gt 0 ]; then
        log_info "Verification passed"
        return 0
    else
        log_error "Verification failed - no tables found"
        return 1
    fi
}

# Main restore process
main() {
    log_info "Starting LinkFlow restore process..."
    
    # Download from S3 if requested
    if [ "$USE_S3" = "true" ]; then
        if [ -z "$RESTORE_DATE" ]; then
            log_error "Date required when restoring from S3 (-d option)"
            exit 1
        fi
        download_from_s3 "$RESTORE_DATE"
    fi
    
    # Restore PostgreSQL
    if [ "$REDIS_ONLY" != "true" ]; then
        if [ ! -z "$RESTORE_FILE" ]; then
            POSTGRES_BACKUP="$RESTORE_FILE"
        else
            POSTGRES_BACKUP=$(find_latest_backup "postgres")
        fi
        
        if [ ! -f "$POSTGRES_BACKUP" ]; then
            log_error "PostgreSQL backup file not found: $POSTGRES_BACKUP"
            exit 1
        fi
        
        restore_postgres "$POSTGRES_BACKUP"
        if [ $? -ne 0 ]; then
            log_error "PostgreSQL restore failed"
            exit 1
        fi
    fi
    
    # Restore Redis
    if [ "$POSTGRES_ONLY" != "true" ]; then
        REDIS_BACKUP=$(find_latest_backup "redis")
        if [ -f "$REDIS_BACKUP" ]; then
            restore_redis "$REDIS_BACKUP"
        else
            log_warning "Redis backup not found, skipping Redis restore"
        fi
    fi
    
    log_info "Restore process completed successfully"
    
    # Show summary
    echo ""
    echo "========================================="
    echo "Restore Summary:"
    echo "  Date: $RESTORE_DATE"
    [ "$REDIS_ONLY" != "true" ] && echo "  PostgreSQL: Restored"
    [ "$POSTGRES_ONLY" != "true" ] && [ -f "$REDIS_BACKUP" ] && echo "  Redis: Restored"
    echo "========================================="
}

# Run main function
main
