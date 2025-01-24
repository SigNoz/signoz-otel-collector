#! /bin/bash

# Exit on error, undefined variables, and pipe failures
set -euo pipefail

# Default schema migrator args
DEFAULT_CLICKHOUSE_DSN="${DEFAULT_CLICKHOUSE_DSN:-tcp://localhost:9000}"
DEFAULT_SCHEMA_MIGRATOR_ARGS="--dsn ${DEFAULT_CLICKHOUSE_DSN} --up="

# Function to show usage
show_usage() {
    cat << EOF
Usage: $0 <sync|async|all> [additional_args]

Arguments:
    sync        Run synchronous migrations
    async       Run asynchronous migrations
    all         Run both sync and async migrations

Additional arguments will be passed to the migrator. If none provided,
default arguments will be used: ${DEFAULT_SCHEMA_MIGRATOR_ARGS}
EOF
}


# Function to run migrations
run_migration() {
    local migration_type=$1
    local additional_args=$2
    echo "Running ${migration_type} migrations..."
    if ! signoz-schema-migrator "${migration_type}" "${additional_args}"; then
        echo "Error: ${migration_type} migration failed"
        exit 1
    fi
}

# Validate input arguments
if [[ $# -eq 0 ]];
    show_usage
    exit 1
fi

migration_type=$1
if [[ "$migration_type" != "sync" && "$migration_type" != "async" && "$migration_type" != "all" ]]; then
    echo "Error: Invalid migration type: ${migration_type}"
    show_usage
    exit 1
fi

shift
additional_args=$@

if [[ "$additional_args" == "" ]]; then
    additional_args="${DEFAULT_SCHEMA_MIGRATOR_ARGS}"
fi

# Main execution
echo "Starting schema migrations with args: ${additional_args}"
if [[ "$migration_type" == "all" ]]; then
    run_migration "sync" "${additional_args}"
    run_migration "async" "${additional_args}"
else
    run_migration "${migration_type}" "${additional_args}"
fi
echo "All migrations completed successfully"
