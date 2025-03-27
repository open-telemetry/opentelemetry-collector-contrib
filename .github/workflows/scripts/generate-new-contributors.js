// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

const simpleGit = require('simple-git');
const git = simpleGit("../../../");

function getUniqueCombinations(data) {
    const uniqueSet = new Set();
    const uniqueArray = [];

    data.forEach(item => {
        const combination = `${item.name}-${item.email}`;
        if (!uniqueSet.has(combination)) {
            uniqueSet.add(combination);
            uniqueArray.push(item);
        }
    });

    return uniqueArray;
}

async function getFirstTimeContributors(fromTag, toTag) {
    try {
        const fromCommit = await git.revparse(fromTag)
        const toCommit = await git.revparse(toTag)
        // Get the list of commits between two tags
        const releaseCommits = await git.log({
            from: fromCommit, to: toCommit,
            format: "oneline"
        }).then(result => result.all);

        const authorsAndHashes = releaseCommits.map(commit => {
            return {name: commit.author_name, email: commit.author_email, hash: commit.hash}
        });

        const uniqueAuthorsAndHashes = getUniqueCombinations(authorsAndHashes);
        const allCommits = await git.log({
            format: "oneline",
        })

        const firstTimeContributorsAndHashes = [];
        for (const item of uniqueAuthorsAndHashes) {
            console.log(`Handling commits of ${item.name}`)
            const authorCommits = allCommits.all.filter(commit => {
                return commit.author_name === item.name && commit.author_email === item.email
            });

            if (authorCommits.length === 1) {
                firstTimeContributorsAndHashes.push(item);
            }
        }

        return firstTimeContributorsAndHashes;
    } catch (error) {
        console.error('Error:', error);
    }
}

async function main({ github }) {
    getFirstTimeContributors('v0.121.0', 'v0.122.0').then(contributors => {
        console.log('First-time contributors:', contributors);
        console.log('Number of first-time contributors: ', contributors.length);
    });
}

module.exports = async ({ github }) => {
    await main({ github })
}

github = undefined
main({ github })
