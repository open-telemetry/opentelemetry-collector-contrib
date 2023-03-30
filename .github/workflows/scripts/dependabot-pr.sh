#!/bin/bash -ex

git config user.name opentelemetrybot
git config user.email 107717825+opentelemetrybot@users.noreply.github.com

PR_NAME=dependabot-prs/`date +'%Y-%m-%dT%H%M%S'`
git checkout -b $PR_NAME

IFS=$'\n'
requests=$( gh pr list --search "author:app/dependabot" --limit 1000 --json title --jq '.[].title' | sort )
message=""

last_updated=""

for line in $requests; do
    if [[ $line != Bump* ]]; then 
        continue
    fi

    module=$(echo $line | cut -f 2 -d " ")
    if [[ $module == go.opentelemetry.io/collector* ]]; then
        continue
    fi
    version=$(echo $line | cut -f 6 -d " ")
    message+=$line
    message+=$'\n'
    if [[ "$last_updated" == "$module $version" ]]; then
        continue
    fi
    last_updated="$module $version"
    make for-all CMD="$GITHUB_WORKSPACE/internal/buildscripts/update-dep" MODULE=$module VERSION=v$version
done

make gotidy
make genotelcontribcol
make genoteltestbedcol
make otelcontribcol

git add go.sum go.mod
git add "**/go.sum" "**/go.mod"
git commit -m "dependabot updates `date`
$message"
git push origin $PR_NAME

gh pr create --title "[chore] dependabot updates `date`" --body "$message"
