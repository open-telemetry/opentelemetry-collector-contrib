#!/bin/bash
# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0


# 1. Check if a directory argument was provided
if [ -z "$1" ]; then
    echo "Usage: $0 <target_directory>"
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

    echo "Run on: $(basename "$dir")"

    (cd "$dir" && make schemagen)
    if [ $? -ne 0 ]; then
        FAILED_DIRS+=("$(basename "$dir")")
    fi

    echo "Done with $(basename "$dir")"
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