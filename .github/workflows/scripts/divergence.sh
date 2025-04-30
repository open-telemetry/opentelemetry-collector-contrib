#!/bin/bash

# Default values
ORIGIN_BRANCH="aws-cwa-dev"
UPSTREAM_BRANCH="release/v0.115.x"
DETAIL_LEVEL=1
SNAPSHOT_FILE=""
COMPARE_MODE="current"  # can be "current" or "compare-snapshot"

# Known/Expected divergences
TRACKED_PATTERNS=(
    "^\.chloggen-aws/"
    "^\.github/workflows/"
)

# Function to capitalize first letter
capitalize() {
    local str="$1"
    printf "%s%s" "$(echo "${str:0:1}" | tr '[:lower:]' '[:upper:]')" "${str:1}"
}

# Function to filter files based on tracked patterns
filter_files() {
    local mode=$1  # "tracked" or "untracked"
    local input_file=$(mktemp)
    cat > "$input_file"

    if [ "$mode" = "tracked" ]; then
        # For tracked files, match any of the patterns
        local pattern=$(IFS="|" ; echo "${TRACKED_PATTERNS[*]}")
        grep -E "$pattern" "$input_file" || true
    else
        # For untracked files, don't match any of the patterns
        local pattern=$(IFS="|" ; echo "${TRACKED_PATTERNS[*]}")
        grep -Ev "$pattern" "$input_file" || true
    fi
    rm "$input_file"
}

# Function to process directories based on detail level
process_directories() {
    local mode=$1  # "tracked" or "untracked"
    local files=$(mktemp)

    # Get the list of files for this mode
    git diff --name-only "upstream/$UPSTREAM_BRANCH...origin/$ORIGIN_BRANCH" | filter_files "$mode" > "$files"

    local title="$(capitalize "$mode") divergence"

    if [ ! -s "$files" ]; then
        echo "No $title found."
        rm "$files"
        return
    fi

    case $DETAIL_LEVEL in
        1)  # Directories only
            echo "$title - Modified directories:"
            echo "-----------------------------------------------------------"
            cat "$files" | \
                xargs -I {} dirname {} 2>/dev/null | \
                sort -u | \
                grep -v "^.$" || true
            ;;

        2)  # Directories with file counts
            echo "$title - Modified directories with file counts:"
            echo "-----------------------------------"
            cat "$files" | \
                awk '{
                    dir=$0
                    gsub(/\/[^\/]*$/, "", dir)
                    if (dir == "") dir="."
                    dirs[dir]++
                }
                END {
                    for (d in dirs) {
                        if (d != ".") printf "%-60s (%d files)\n", d, dirs[d]
                    }
                }' | sort
            ;;

        3)  # Directories with file lists
            echo "$title - Modified directories and files:"
            echo "---------------------------"
            cat "$files" | \
                awk '{
                    dir=$0
                    gsub(/\/[^\/]*$/, "", dir)
                    if (dir == "") dir="."
                    if (!(dir in printed) && dir != ".") {
                        print "\n" dir ":"
                        printed[dir] = 1
                    }
                    if (dir != ".") print "  - " $0
                }' | sed '/^$/d'
            ;;
    esac

    rm "$files"
}

# Function to get current diff files
get_current_diff_files() {
    git diff --name-only "upstream/$UPSTREAM_BRANCH...origin/$ORIGIN_BRANCH"
}

# Function to save snapshot
save_snapshot() {
    local snapshot_file="$1"
    local temp_file=$(mktemp)

    echo "# Snapshot created on $(date)" > "$snapshot_file"
    echo "# Origin: $ORIGIN_BRANCH" >> "$snapshot_file"
    echo "# Upstream: $UPSTREAM_BRANCH" >> "$snapshot_file"
    echo "" >> "$snapshot_file"

    get_current_diff_files > "$temp_file"

    # Save tracked and untracked files separately
    echo "### TRACKED FILES ###" >> "$snapshot_file"
    cat "$temp_file" | filter_files "tracked" >> "$snapshot_file"

    echo -e "\n### UNTRACKED FILES ###" >> "$snapshot_file"
    cat "$temp_file" | filter_files "untracked" >> "$snapshot_file"

    rm "$temp_file"
    echo "Snapshot saved to: $snapshot_file"
}

# Function to compare with snapshot
# Function to compare with snapshot
compare_with_snapshot() {
    local snapshot_file="$1"
    local current_tracked=$(mktemp)
    local current_untracked=$(mktemp)
    local snapshot_tracked=$(mktemp)
    local snapshot_untracked=$(mktemp)

    # Get current diff files and separate them
    echo "Getting current state..."
    get_current_diff_files | filter_files "tracked" | sort > "$current_tracked"
    get_current_diff_files | filter_files "untracked" | sort > "$current_untracked"

    # Extract files from snapshot
    echo "Reading snapshot state..."
    sed -n '/^### TRACKED FILES ###/,/^### UNTRACKED FILES ###/p' "$snapshot_file" | \
        grep -v "^###" | grep -v "^$" | sort > "$snapshot_tracked"

    sed -n '/^### UNTRACKED FILES ###/,$p' "$snapshot_file" | \
        grep -v "^###" | grep -v "^$" | sort > "$snapshot_untracked"

    echo
    echo "NEW DIVERGENCES SINCE SNAPSHOT"
    echo "============================="

    # Compare for new divergences
    local new_tracked=$(mktemp)
    local new_untracked=$(mktemp)
    comm -23 "$current_tracked" "$snapshot_tracked" > "$new_tracked"
    comm -23 "$current_untracked" "$snapshot_untracked" > "$new_untracked"

    if [ ! -s "$new_tracked" ] && [ ! -s "$new_untracked" ]; then
        echo "No new divergences found since snapshot."
    else
        if [ -s "$new_tracked" ]; then
            echo "TRACKED FILES:"
            echo "-------------"
            echo "Current tracked files: $(wc -l < "$current_tracked")"
            echo "Snapshot tracked files: $(wc -l < "$snapshot_tracked")"
            echo "New tracked files that have diverged:"
            cat "$new_tracked" | sed 's/^/  /'
            echo
        fi

        if [ -s "$new_untracked" ]; then
            echo "UNTRACKED FILES:"
            echo "---------------"
            echo "Current untracked files: $(wc -l < "$current_untracked")"
            echo "Snapshot untracked files: $(wc -l < "$snapshot_untracked")"
            echo "New untracked files that have diverged:"
            cat "$new_untracked" | sed 's/^/  /'
            echo
        fi
    fi

    # Compare for resolved divergences
    echo "RESOLVED DIVERGENCES SINCE SNAPSHOT"
    echo "================================="

    local resolved_tracked=$(mktemp)
    local resolved_untracked=$(mktemp)
    comm -13 "$current_tracked" "$snapshot_tracked" > "$resolved_tracked"
    comm -13 "$current_untracked" "$snapshot_untracked" > "$resolved_untracked"

    if [ ! -s "$resolved_tracked" ] && [ ! -s "$resolved_untracked" ]; then
        echo "No resolved divergences found since snapshot."
    else
        if [ -s "$resolved_tracked" ]; then
            echo "TRACKED FILES:"
            echo "-------------"
            echo "Files no longer divergent:"
            cat "$resolved_tracked" | sed 's/^/  /'
            echo
        fi

        if [ -s "$resolved_untracked" ]; then
            echo "UNTRACKED FILES:"
            echo "---------------"
            echo "Files no longer divergent:"
            cat "$resolved_untracked" | sed 's/^/  /'
            echo
        fi
    fi

    # Clean up temporary files
    rm "$current_tracked" "$current_untracked" "$snapshot_tracked" "$snapshot_untracked" \
       "$new_tracked" "$new_untracked" "$resolved_tracked" "$resolved_untracked"
}

# Modified process_directories to accept input from file
process_directories_from_file() {
    local mode=$1
    local files=$(mktemp)

    # Filter the input based on mode
    filter_files "$mode" > "$files"

    local title="$(capitalize "$mode") divergence"

    if [ ! -s "$files" ]; then
        echo "No new $title since snapshot."
        rm "$files"
        return
    fi

    case $DETAIL_LEVEL in
        1)  # Directories only
            echo "$title - Modified directories:"
            echo "-----------------------------------------------------------"
            cat "$files" | \
                xargs -I {} dirname {} 2>/dev/null | \
                sort -u | \
                grep -v "^.$" || true
            ;;

        2)  # Directories with file counts
            echo "$title - Modified directories with file counts:"
            echo "-----------------------------------"
            cat "$files" | \
                awk '{
                    dir=$0
                    gsub(/\/[^\/]*$/, "", dir)
                    if (dir == "") dir="."
                    dirs[dir]++
                }
                END {
                    for (d in dirs) {
                        if (d != ".") printf "%-60s (%d files)\n", d, dirs[d]
                    }
                }' | sort
            ;;

        3)  # Directories with file lists
            echo "$title - Modified directories and files:"
            echo "---------------------------"
            cat "$files" | \
                awk '{
                    dir=$0
                    gsub(/\/[^\/]*$/, "", dir)
                    if (dir == "") dir="."
                    if (!(dir in printed) && dir != ".") {
                        print "\n" dir ":"
                        printed[dir] = 1
                    }
                    if (dir != ".") print "  - " $0
                }' | sed '/^$/d'
            ;;
    esac

    rm "$files"
}

# Usage function
usage() {
    echo "Usage: $0 [-o origin_branch] [-u upstream_branch] [-d detail_level] [-s snapshot_file] [-c]"
    echo "  -o: Origin branch name (default: aws-cwa-dev)"
    echo "  -u: Upstream branch name (default: main)"
    echo "  -d: Detail level (1: directories only, 2: with file counts, 3: with full file lists)"
    echo "  -s: Snapshot file path (for saving or comparing)"
    echo "  -c: Compare mode (compare current diff with snapshot)"
    echo
    echo "Examples:"
    echo "  Create snapshot:     $0 -s /path/to/snapshot.txt"
    echo "  Compare to snapshot: $0 -s /path/to/snapshot.txt -c"
    exit 1
}

# Parse command line options
while getopts "o:u:d:s:ch" opt; do
    case $opt in
        o) ORIGIN_BRANCH="$OPTARG";;
        u) UPSTREAM_BRANCH="$OPTARG";;
        d) DETAIL_LEVEL="$OPTARG";;
        s) SNAPSHOT_FILE="$OPTARG";;
        c) COMPARE_MODE="compare-snapshot";;
        h) usage;;
        ?) usage;;
    esac
done

# Main execution
if [ -n "$SNAPSHOT_FILE" ]; then
    if [ "$COMPARE_MODE" = "compare-snapshot" ]; then
        if [ ! -f "$SNAPSHOT_FILE" ]; then
            echo "Error: Snapshot file does not exist: $SNAPSHOT_FILE"
            exit 1
        fi
        compare_with_snapshot "$SNAPSHOT_FILE"
    else
        save_snapshot "$SNAPSHOT_FILE"
    fi
    exit 0
fi

# Ensure we're in a git repository
if ! git rev-parse --git-dir > /dev/null 2>&1; then
    echo "Error: Not in a git repository"
    exit 1
fi

# Update remotes
echo "Fetching from remotes..."
git fetch origin
git fetch upstream

# Verify branches exist
if ! git show-ref --verify --quiet "refs/remotes/upstream/$UPSTREAM_BRANCH"; then
    echo "Error: Branch $UPSTREAM_BRANCH does not exist in upstream"
    exit 1
fi

if ! git show-ref --verify --quiet "refs/remotes/origin/$ORIGIN_BRANCH"; then
    echo "Error: Branch $ORIGIN_BRANCH does not exist in origin"
    exit 1
fi

# Show comparison info
echo "Comparing upstream/$UPSTREAM_BRANCH with origin/$ORIGIN_BRANCH"
echo "============================================================"
total_files=$(git diff --name-only "upstream/$UPSTREAM_BRANCH...origin/$ORIGIN_BRANCH" | wc -l)
echo "Total changed files: $total_files"
echo

# Process untracked divergence
echo "UNTRACKED DIVERGENCE"
echo "==================="
process_directories "untracked"
echo

# Process tracked divergence
echo "TRACKED DIVERGENCE"
echo "================="
process_directories "tracked"

# Show change size summary for detail levels 2 and 3
if [ "$DETAIL_LEVEL" = "2" ] || [ "$DETAIL_LEVEL" = "3" ]; then
    echo
    echo "Change size summary:"
    echo "------------------"
    git diff --stat "upstream/$UPSTREAM_BRANCH...origin/$ORIGIN_BRANCH" | tail -n 1
fi