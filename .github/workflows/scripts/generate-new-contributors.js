// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

import { simpleGit } from 'simple-git';
const git = simpleGit();

const REPO_NAME = "opentelemetry-collector-contrib"
const REPO_OWNER = "open-telemetry"

const firstTimeContributorText = "We are thrilled to welcome our new first-time contributors to this project. Thank you for your contributions "

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

function generateNewContributorText(newContributors) {
    const annotatedUsernames = newContributors.map(username => "@" + username)
    return firstTimeContributorText + annotatedUsernames.join(", ") + " ! 🎉"
}

export const main = async (github, tag, previous_tag) => {
    const newContributorsAndHashes = await getFirstTimeContributors(previous_tag, tag)

    if(newContributorsAndHashes.length === 0) {
        return ""
    }

    const usernames = [];
    for (const contributor of newContributorsAndHashes) {
        const response = await github.request('GET /repos/{owner}/{repo}/commits/{ref}', {
            owner: REPO_OWNER,
            repo: REPO_NAME,
            ref: contributor.hash,
            headers: {
                'X-GitHub-Api-Version': '2022-11-28'
            }
        })
        usernames.push(response.data.author.login)
    }
    console.log('First-time contributors:', usernames);
    console.log('Number of first-time contributors: ', usernames.length);

    return generateNewContributorText(usernames)
}

export default async function (github, tag, previous_tag) {
    return await main( github, tag, previous_tag)
}
