// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

const simpleGit = require("simple-git");

const REPO_NAME = "opentelemetry-collector-contrib"
const REPO_OWNER = "open-telemetry"
const git = simpleGit("../../../")


async function main({github, context}) {
    let fromCommit = await git.revparse("v0.121.0")
    let toCommit = await git.revparse("v0.122.0")
    let commits = await git.log({
        from: fromCommit,
        to: toCommit,
        format: "oneline"
    })
    console.log(`Got ${commits.all.length} commits since last release.`)

    let commitAuthors = commits.all.map(commit => {
        return {authorName: commit.author_name, authorEmail: commit.author_email}
    })

    let firstTimeContributors = commitAuthors.filter(async author => {
        let firstTimer = await simpleGit("../../../").raw([
            'rev-list',
            'HEAD',
            '--count',
            '--author=' + author.authorName,
        ])
        firstTimer = firstTimer.trim()

        return firstTimer === '1'
    })
    
    console.log(firstTimeContributors)
    console.log("bla")
}

main({undefined, undefined}).then(r => {console.log("done")})
