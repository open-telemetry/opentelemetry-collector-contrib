#!/bin/bash
# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0


# 1. Parse CLI arguments
usage() {
    echo "Usage: $0 [-i dir1,dir2,...] <target_directory>"
}

IGNORED_DIRS=()
while getopts ":i:h" opt; do
    case $opt in
        i)
            if [ -n "$OPTARG" ]; then
                IFS=',' read -r -a tmp <<< "$OPTARG"
                IGNORED_DIRS+=("${tmp[@]}")
            fi
            ;;
        h)
            usage
            exit 0
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

# 3. Check if the provided path is actually a directory
if [ ! -d "$TARGET_DIR" ]; then
    echo "Error: $TARGET_DIR is not a directory."
    exit 1
fi

echo "Processing subdirectories in: $TARGET_DIR"
echo "------------------------------------------"

# 4. Loop through subdirectories (one level only)
for dir in "$TARGET_DIR"/*/; do

    # Check if any directories exist to avoid errors with empty globs
    [ -e "$dir" ] || continue

    # Remove the trailing slash for cleaner output
    dir=${dir%/}
    dir_basename=$(basename "$dir")

    skip_dir=false
    for ignored in "${IGNORED_DIRS[@]}"; do
        if [[ "$dir_basename" == "$ignored" ]]; then
            skip_dir=true
            break
        fi
    done
    if [ "$skip_dir" = true ]; then
        echo "Skipping $dir_basename (in ignore list)"
        echo "------------------------------------------"
        continue
    fi

    echo "Run on: $dir_basename"

    (cd "$dir" && make schemagen)
    if [ $? -ne 0 ]; then
        FAILED_DIRS+=("$dir_basename")
    fi

    echo "Done with $dir_basename"
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
