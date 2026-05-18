#!/usr/bin/env bash
set -euo pipefail

# ======================================================================================
# Runs one shard of the build-and-test-experimental "exp-checks" matrix.
#
# The original `checks` job in build-and-test.yml runs 18 codegen + validation
# targets serially. Splitting them into matrix shards lets a fresh runner
# handle each subset so the file-modifying targets (goporto, gogci, gotidy,
# crosslink, generate) don't share a working tree.
#
# Usage:
#   ./run-checks-shard.sh <shard-name>
#
# Shards:
#   codegen                -- `make generate` + git-clean check
#   porto-and-gci          -- goporto + gogci (both write to source files)
#   go-mod-hygiene         -- crosslink + tidylist + gotidy (all touch go.mod/go.sum)
#   small-generators       -- gendistributions + genlabels + gencodecov
#   schemas-and-templates  -- generate-gh-issue-templates + generate-schemas
#                             + generate-chloggen-components
#   read-only-checks       -- checkdoc + checkmetadata + checkapi + multimod-verify
#
# Each generator step is followed by a `git diff --exit-code` so the shard
# fails if the generated artifact would change.
#
# Run locally with: bash .github/workflows/scripts/run-checks-shard.sh <shard>
# ======================================================================================

if [[ $# -ne 1 ]]; then
  echo "usage: $0 <shard-name>" >&2
  exit 2
fi

shard="$1"

case "${shard}" in
  codegen)
    make generate
    if [[ -n $(git status -s) ]]; then
      echo 'Generated code is out of date, please run "make generate" and commit the changes in this PR.'
      exit 1
    fi
    ;;
  porto-and-gci)
    make -j2 goporto
    git diff --exit-code || { echo 'Porto links are out of date, please run "make goporto" and commit the changes in this PR.'; exit 1; }
    make gogci
    git diff --exit-code || { echo 'go package imports not formatted, please run "make gogci" and commit the changes in this PR.'; exit 1; }
    ;;
  go-mod-hygiene)
    make crosslink
    git diff --exit-code || { echo 'Replace statements are out of date, please run "make crosslink" and commit the changes in this PR.'; exit 1; }
    make tidylist
    git diff --exit-code || { echo 'Tidylist is out of date, please run "make tidylist" and commit the changes in this PR.'; exit 1; }
    make gotidy
    git diff --exit-code || { echo 'go.mod/go.sum deps changes detected, please run "make gotidy" and commit the changes in this PR.'; exit 1; }
    ;;
  small-generators)
    make gendistributions
    git diff -s --exit-code || { echo 'Generated code is out of date, please run "make gendistributions" and commit the changes in this PR.'; exit 1; }
    make genlabels
    git diff -s --exit-code || { echo '.github/component_labels.txt is out of date, please run "make genlabels" and commit the changes in this PR.'; exit 1; }
    make gencodecov
    git --no-pager diff
    git diff -s --exit-code '.codecov.yml' || { echo '.codecov.yml is out of date, please run "make gencodecov" and commit the changes in this PR.'; exit 1; }
    ;;
  schemas-and-templates)
    make generate-gh-issue-templates
    git diff --exit-code '.github/ISSUE_TEMPLATE' || { echo 'Dropdowns in issue templates are out of date, please run "make generate-gh-issue-templates" and commit the changes in this PR.'; exit 1; }
    make generate-schemas
    git diff --exit-code '*.schema.yaml' || { echo 'Config schemas are out of date, please run "make generate-schemas" and commit the changes in this PR.'; exit 1; }
    make generate-chloggen-components
    git diff --exit-code || { echo '.chloggen/config.yaml is out of date, please run "make generate-chloggen-components" and commit the changes.'; exit 1; }
    ;;
  read-only-checks)
    make checkdoc
    make checkmetadata
    make checkapi
    make multimod-verify
    ;;
  *)
    echo "Unknown shard: ${shard}" >&2
    echo "Known shards: codegen porto-and-gci go-mod-hygiene small-generators schemas-and-templates read-only-checks" >&2
    exit 2
    ;;
esac
