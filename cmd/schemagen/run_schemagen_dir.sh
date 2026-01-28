#!/bin/bash
# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0


# 1. Parse CLI arguments
usage() {
    echo "Usage: $0 [-v] [-m] [-i dir1,dir2,...] <target_directory>"
    echo "  -i  Comma-separated directory basenames to ignore (recursion prunes them)"
    echo "  -m  After successful run also run schemagen in internal/metadata"
    echo "  -v  Validation mode - process only directories containing *.schema.yaml files"
}

IGNORED_DIRS=()
RECURSIVE_MODE=false
METADATA_MODE=false
while getopts ":i:hmv" opt; do
    case $opt in
        i)
            if [ -n "$OPTARG" ]; then
                IFS=',' read -r -a tmp <<< "$OPTARG"
                IGNORED_DIRS+=("${tmp[@]}")
            fi
            ;;
        m)
            METADATA_MODE=true
            ;;
        h)
            usage
            exit 0
            ;;
        v)
            RECURSIVE_MODE=true
            ;;
        \?)
            echo "Invalid option: -$OPTARG"
            usage
            exit 1
            ;;
    esac
done
shift $((OPTIND - 1))

if [ -z "$1" ]; then
    usage
    exit 1
fi

# 2. Assign input to a variable and resolve absolute path
TARGET_DIR=$(realpath "$1")
FAILED_DIRS=() # Array to keep track of failures
dirs_to_process=()
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
SCHEMAGEN_BIN="$REPO_ROOT/.tools/schemagen"

if [ ! -x "$SCHEMAGEN_BIN" ]; then
    echo "Error: schemagen binary not found or not executable at $SCHEMAGEN_BIN"
    exit 1
fi

run_schemagen_for_dir() {
    local dir_path=$1
    (cd "$dir_path" && "$SCHEMAGEN_BIN")
}

dir_already_scheduled() {
    local candidate=$1
    for existing_dir in "${dirs_to_process[@]}"; do
        if [[ "$existing_dir" == "$candidate" ]]; then
            return 0
        fi
    done
    return 1
}

format_dir_display() {
    local dir_path=$1
    local base
    base=$(basename "$dir_path")
    if [ "$RECURSIVE_MODE" = true ]; then
        if [[ "$dir_path" == "$TARGET_DIR" ]]; then
            printf '.\n'
        elif [[ "$dir_path" == "$TARGET_DIR"/* ]]; then
            local offset=$(( ${#TARGET_DIR} + 1 ))
            printf '%s\n' "${dir_path:$offset}"
        else
            printf '%s\n' "$base"
        fi
    else
        printf '%s\n' "$base"
    fi
}

# 3. Check if the provided path is actually a directory
if [ ! -d "$TARGET_DIR" ]; then
    echo "Error: $TARGET_DIR is not a directory."
    exit 1
fi

echo "------------------------------------------"

if [ "$RECURSIVE_MODE" = true ]; then
    find_args=(find "$TARGET_DIR")
    if [ ${#IGNORED_DIRS[@]} -gt 0 ]; then
        find_args+=('(' '-type' 'd' '(')
        for idx in "${!IGNORED_DIRS[@]}"; do
            find_args+=('-name' "${IGNORED_DIRS[$idx]}")
            if [ "$idx" -lt $(( ${#IGNORED_DIRS[@]} - 1 )) ]; then
                find_args+=('-o')
            fi
        done
        find_args+=(')' '-prune' ')' '-o')
    fi
    find_args+=('-type' 'f' '-name' '*.schema.yaml' '-print0')

    while IFS= read -r -d '' schema_file; do
        dir_path=$(dirname "$schema_file")
        if dir_already_scheduled "$dir_path"; then
            continue
        fi
        dirs_to_process+=("$dir_path")
    done < <("${find_args[@]}")
else
    for dir in "$TARGET_DIR"/*/; do
        [ -e "$dir" ] || continue
        dir=${dir%/}
        dirs_to_process+=("$dir")
    done
fi

# 4. Loop through gathered directories
for dir in "${dirs_to_process[@]}"; do
    dir_basename=$(basename "$dir")
    dir_display=$(format_dir_display "$dir")

    skip_dir=false
    for ignored in "${IGNORED_DIRS[@]}"; do
        if [[ "$dir_basename" == "$ignored" ]]; then
            skip_dir=true
            break
        fi
    done
    if [ "$skip_dir" = true ]; then
        echo "Skipping $dir_display (in ignore list)"
        echo "------------------------------------------"
        continue
    fi

    echo "Run on: $dir_display"

    if ! run_schemagen_for_dir "$dir"; then
        FAILED_DIRS+=("$dir_display")
        echo "Done with $dir_display"
        echo "------------------------------------------"
        continue
    fi

    echo "Done with $dir_display"
    if [ "$METADATA_MODE" = true ]; then
        metadata_dir="$dir/internal/metadata"
        if [ -f "$metadata_dir/generated_config.go" ]; then
            metadata_basename=$(basename "$metadata_dir")
            skip_metadata=false
            for ignored in "${IGNORED_DIRS[@]}"; do
                if [[ "$metadata_basename" == "$ignored" ]]; then
                    skip_metadata=true
                    break
                fi
            done
            metadata_display="$dir_display/internal/metadata"
            if [ "$skip_metadata" = true ]; then
                echo "Skipping $metadata_display (in ignore list)"
            else
                echo "Run on: $metadata_display"
                if ! run_schemagen_for_dir "$metadata_dir"; then
                    FAILED_DIRS+=("$metadata_display")
                fi
                echo "Done with $metadata_display"
            fi
        fi
    fi

    echo "------------------------------------------"
done

if [ ${#FAILED_DIRS[@]} -ne 0 ]; then
    echo "Script finished with errors in the following directories:"
    for f in "${FAILED_DIRS[@]}"; do
        echo " - $f"
    done
    exit 1
else
    echo "All tasks completed successfully!"
    exit 0
fi
