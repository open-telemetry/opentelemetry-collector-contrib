#!/usr/bin/env bash
set -euo pipefail

# ======================================================================================
# Optimizes CI by calculating the minimum set of modules that need testing based
# on the files changed in a Pull Request. It outputs a JSON matrix for GitHub Actions.
# The logic goes like this:
# 1. If 'ci:full' label is present OR event is not a PR -> Full Matrix.
# 2. If core build files (Makefile, workflows) changed -> Full Matrix.
# 3. Identifies Go modules directly modified by the git diff.
# 4. Recursively finds modules that depend on the modified modules
#    (e.g., if 'pkg/ottl' changes, we must test 'processor/filterprocessor').
# 5. Groups the resulting modules into a fixed number of "buckets"
#    (MAX_JOBS) to prevent spawning hundreds of concurrent CI jobs.
#
#
# Environment variables:
#   FULL_MATRIX (Required in CI): JSON array of the fallback/full group list.
#   PR_HEAD     (Optional): SHA of the PR tip. Defaults to 'HEAD' locally.
#   BASE_REF    (Optional): Branch target. Defaults to 'main' locally.
#   MAX_JOBS    (Optional): Max concurrent buckets. Defaults to 10.
#   FORCE_FULL  (Optional): Set to "true" to force full matrix output.
# ======================================================================================

# Ensure we are in the repo root
repo_root="$(git rev-parse --show-toplevel 2>/dev/null || true)"
if [[ -z "${repo_root}" ]]; then
  echo "error: must be run inside a git repo" >&2
  exit 2
fi
cd "${repo_root}"

# -----------------------------------------------------------------------------
# Define local variables
# -----------------------------------------------------------------------------
# If not running in GitHub Actions, set defaults for local testing
if [[ -z "${GITHUB_ACTIONS:-}" ]]; then
  echo ":: Local detection: Running in LOCAL mode" >&2
  export EVENT_NAME="pull_request"
  export BASE_REF="${BASE_REF:-main}" # Default to comparing against main
  export PR_HEAD="${PR_HEAD:-HEAD}"
  export GITHUB_OUTPUT="/dev/stdout" # Print results to console

  # Mock FULL_MATRIX if not provided locally
  if [[ -z "${FULL_MATRIX:-}" ]]; then
    FULL_MATRIX='["receiver-0","receiver-1","receiver-2","receiver-3","processor-0","processor-1","exporter-0","exporter-1","exporter-2","exporter-3","extension","connector","internal","pkg","cmd-0","other"]'
  fi
else
  # In CI, FULL_MATRIX is mandatory
  : "${FULL_MATRIX:?The FULL_MATRIX environment variable is required}"
fi

# Config
MAX_JOBS=${MAX_JOBS:-10}
GITHUB_OUTPUT="${GITHUB_OUTPUT:-/dev/stdout}"

# -----------------------------------------------------------------------------
# Helper functions
# -----------------------------------------------------------------------------
exit_full_mode() {
  echo "Reason: $1" >&2
  echo "Selecting FULL Matrix." >&2
  echo "matrix=${FULL_MATRIX}" >> "$GITHUB_OUTPUT"
  exit 0
}

exit_scoped_mode() {
  local matrix_json="$1"
  local count="$2"
  echo "Reason: Scoped changes detected ($count modules affected)." >&2
  echo "Computed Matrix: ${matrix_json}" >&2
  echo "matrix=${matrix_json}" >> "$GITHUB_OUTPUT"
  exit 0
}

# -----------------------------------------------------------------------------
# Check if script should keep running
# -----------------------------------------------------------------------------

# Manual label override
if [[ "${FORCE_FULL:-}" == "true" ]]; then
  exit_full_mode "Manual override detected (Label: 'ci:full')."
fi

# Non-PR Events (Push, Schedule, Merge Group)
# Note: In local mode, we forced this to 'pull_request' above so this passes.
if [[ "${EVENT_NAME:-}" != "pull_request" ]]; then
  exit_full_mode "Non-PR event (${EVENT_NAME:-unknown})."
fi

# -----------------------------------------------------------------------------
# Check what changed
# -----------------------------------------------------------------------------
echo "Calculating git diff against origin/${BASE_REF}..." >&2

# Fetch origin/main locally if needed to ensure comparison works
if [[ -z "${GITHUB_ACTIONS:-}" ]]; then
  git fetch origin "${BASE_REF}" >/dev/null 2>&1 || true
fi

# Calculate the merge base
base_sha="$(git merge-base "origin/${BASE_REF}" "${PR_HEAD}")"
changed_files="$(git diff --name-only --diff-filter=ACMRTUXB "${base_sha}" "${PR_HEAD}")"

if [[ -z "${changed_files}" ]]; then
  echo "No changed files detected." >&2
  exit_scoped_mode "[]" 0
fi

# -----------------------------------------------------------------------------
# Check if there are changes to infrastructure files
# -----------------------------------------------------------------------------
infra_regex='^(Makefile|Makefile\.Common|Makefile\.Weaver|\.golangci\.ya?ml|\.github/workflows/|internal/tools/|internal/buildscripts/|versions\.yaml|distributions\.yaml|renovate\.json|\.codecov\.yml)'

if echo "${changed_files}" | grep -Eq "$infra_regex"; then
  exit_full_mode "Critical infrastructure files changed."
fi

# -----------------------------------------------------------------------------
# Get dependencies of the modules changed
# -----------------------------------------------------------------------------
find_module_dir_for_file() {
  local d
  d="$(dirname "$1")"
  [[ "$d" == "." ]] && d=""
  while [[ "$d" != "." && "$d" != "/" ]]; do
    if [[ -f "${d}/go.mod" ]]; then
      echo "${d}" | sed 's|^\./||'; return 0
    fi
    d="$(dirname "${d}")"
  done
  if [[ -f "go.mod" ]]; then echo "."; fi
}

read_module_name() {
  local mod_file="$1/go.mod"
  [[ "$1" == "." ]] && mod_file="go.mod"
  if [[ -f "$mod_file" ]]; then
    awk '$1=="module"{print $2; exit}' "$mod_file"
  fi
}

declare -A affected_dirs

# Map files to modules
while IFS= read -r f; do
  [[ -z "${f}" ]] && continue
  f="${f#./}"
  mod_dir="$(find_module_dir_for_file "${f}")"
  [[ -n "${mod_dir}" ]] && affected_dirs["${mod_dir}"]=1
done <<< "${changed_files}"

if [[ ${#affected_dirs[@]} -eq 0 ]]; then
  exit_scoped_mode "[]" 0
fi

# Resolve Transitive Dependencies
all_go_mods="$(find . -type f -name "go.mod" -not -path "*/.git/*" -not -path "*/vendor/*")"

while true; do
  found_new=false
  grep_patterns="$(mktemp)"

  for dir in "${!affected_dirs[@]}"; do
    mod_name="$(read_module_name "${dir}")"
    if [[ -n "${mod_name}" ]]; then
      mod_name_escaped="${mod_name//./\\.}"
      echo "${mod_name_escaped}[[:space:]]" >> "$grep_patterns"
    fi
  done

  if [[ -s "$grep_patterns" ]]; then
    dependents=$(echo "$all_go_mods" | xargs grep -l -f "$grep_patterns" || true)
    if [[ -n "${dependents}" ]]; then
       while IFS= read -r dependent_mod_file; do
        dep_dir="$(dirname "${dependent_mod_file}")"
        dep_dir="${dep_dir#./}"
        [[ "${dep_dir}" == "." ]] && dep_dir="."
        if [[ -z "${affected_dirs[${dep_dir}]+x}" ]]; then
          affected_dirs["${dep_dir}"]=1
          found_new=true
        fi
      done <<< "${dependents}"
    fi
  fi
  rm -f "$grep_patterns"
  [[ "${found_new}" == "false" ]] && break
done

# -----------------------------------------------------------------------------
# Bucket results so we can launch multiple workers on CI
# -----------------------------------------------------------------------------
declare -a buckets
for ((i=0; i<MAX_JOBS; i++)); do buckets[$i]=""; done

idx=0
for dir in "${!affected_dirs[@]}"; do
  bucket_id=$((idx % MAX_JOBS))
  if [[ -z "${buckets[$bucket_id]}" ]]; then
    buckets[$bucket_id]="${dir}"
  else
    buckets[$bucket_id]="${buckets[$bucket_id]} ${dir}"
  fi
  idx=$((idx + 1))
done

# Construct JSON Array
json_array="["
first=true
for bucket in "${buckets[@]}"; do
  if [[ -n "${bucket}" ]]; then
    [[ "$first" == "true" ]] && first=false || json_array+=","
    json_array+="\"${bucket}\""
  fi
done
json_array+="]"

exit_scoped_mode "${json_array}" "${#affected_dirs[@]}"