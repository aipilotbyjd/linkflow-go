#!/usr/bin/env bash
# wait-for-it.sh: Wait for a service to be ready

TIMEOUT=15
QUIET=0
HOST=""
PORT=""

echoerr() { 
    if [[ $QUIET -ne 1 ]]; then 
        echo "$@" 1>&2; 
    fi 
}

usage() {
    cat << USAGE >&2
Usage:
    $0 host:port [-t timeout] [-- command args]
    -h HOST | --host=HOST       Host or IP under test
    -p PORT | --port=PORT       TCP port under test
    -t TIMEOUT | --timeout=TIMEOUT
                                Timeout in seconds, zero for no timeout
    -q | --quiet                Don't output any status messages
    -- COMMAND ARGS             Execute command with args after the test finishes
USAGE
    exit 1
}

wait_for() {
    if [[ $TIMEOUT -gt 0 ]]; then
        echoerr "Waiting $TIMEOUT seconds for $HOST:$PORT"
    else
        echoerr "Waiting for $HOST:$PORT without a timeout"
    fi
    
    start_ts=$(date +%s)
    while :
    do
        if [[ $TIMEOUT -gt 0 ]]; then
            now_ts=$(date +%s)
            elapsed=$((now_ts - start_ts))
            if [[ $elapsed -ge $TIMEOUT ]]; then
                echoerr "Timeout occurred after waiting $TIMEOUT seconds for $HOST:$PORT"
                return 1
            fi
        fi
        
        nc -z "$HOST" "$PORT" >/dev/null 2>&1
        result=$?
        if [[ $result -eq 0 ]]; then
            echoerr "$HOST:$PORT is available"
            return 0
        fi
        sleep 1
    done
}

# Process arguments
while [[ $# -gt 0 ]]
do
    case "$1" in
        *:* )
        HOST=$(echo "$1" | cut -d : -f 1)
        PORT=$(echo "$1" | cut -d : -f 2)
        shift 1
        ;;
        -h)
        HOST="$2"
        shift 2
        ;;
        --host=*)
        HOST="${1#*=}"
        shift 1
        ;;
        -p)
        PORT="$2"
        shift 2
        ;;
        --port=*)
        PORT="${1#*=}"
        shift 1
        ;;
        -t)
        TIMEOUT="$2"
        shift 2
        ;;
        --timeout=*)
        TIMEOUT="${1#*=}"
        shift 1
        ;;
        -q | --quiet)
        QUIET=1
        shift 1
        ;;
        --)
        shift
        break
        ;;
        --help)
        usage
        ;;
        *)
        echoerr "Unknown argument: $1"
        usage
        ;;
    esac
done

if [[ "$HOST" == "" || "$PORT" == "" ]]; then
    echoerr "Error: you need to provide a host and port to test."
    usage
fi

wait_for

RESULT=$?
if [[ $RESULT -ne 0 ]]; then
    exit $RESULT
fi

# Execute command if provided
if [[ $# -gt 0 ]]; then
    exec "$@"
fi
