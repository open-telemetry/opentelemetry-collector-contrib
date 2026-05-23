// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

import { simpleGit } from 'simple-git';
const git = simpleGit();

const REPO_NAME = "opentelemetry-collector-contrib"
const REPO_OWNER = "open-telemetry"

const firstTimeContributorText = "We are thrilled to welcome our first-time contributors to this project. Thank you for your contributions "

function getUniqueCombinations(data) {
    const uniqueSet = new Set();
    const uniqueArray = [];

    data.forEach(item => {
        if (!uniqueSet.has(item.name)) {
            uniqueSet.add(item.name);
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

        const releaseAuthorsAndHashes = releaseCommits.map(commit => {
            return {name: commit.author_name, hash: commit.hash}
        });

        const uniqueAuthorsAndHashes = getUniqueCombinations(releaseAuthorsAndHashes);
        const allCommits = await git.log({
            format: "oneline",
        }).then(result => result.all);

        const firstTimeContributorsAndHashes = [];
        for (const item of uniqueAuthorsAndHashes) {
            const authorCommits = allCommits.filter(commit => commit.author_name === item.name);

            if (authorCommits.length === 1) {
                firstTimeContributorsAndHashes.push(item);
            } else if(authorCommits.length > 1) {
                if(areAllCommitsInArray(authorCommits, releaseCommits)) {
                    firstTimeContributorsAndHashes.push(item);
                }
            }
        }

        return firstTimeContributorsAndHashes;
    } catch (error) {
        console.error('Error:', error);
    }
}

/**
 * areAllCommitsInArray checks if all items in arrayToCheck are also contained in arrayToSearchIn.
 * Comparison is done using the value of item.hash for both arrays
 *
 * @param arrayToCheck items here are looked for in arrayToSearchIn
 * @param arrayToSearchIn is searched for containment checks
 * @returns true if all items in arrayToCheck are also in array 2
 */
function areAllCommitsInArray(arrayToCheck, arrayToSearchIn) {
    return arrayToCheck.every(item => arrayToSearchIn.findIndex(item2 => item2.hash === item.hash) > -1);
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
