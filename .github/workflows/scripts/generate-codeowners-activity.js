// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Generates a monthly report measuring code owner responsiveness: each code owner must have
 * reviewed/replied to at least 80% of the PRs and issues where they were the code owner
 * for the component. Uses the same component labels as the weekly report (e.g. receiver/kubeletstats).
 */

const fs = require('fs');
const path = require('path');

const REPO_OWNER = 'open-telemetry';
const REPO_NAME = 'opentelemetry-collector-contrib';
const RESPONSE_THRESHOLD_PCT = 80;

function debug(msg) {
  console.log(JSON.stringify(msg, null, 2));
}

function genLookbackDates() {
  const now = new Date();
  const midnightYesterday = new Date(
    Date.UTC(
      now.getUTCFullYear(),
      now.getUTCMonth(),
      now.getUTCDate(),
      0,
      0,
      0,
      0
    )
  );
  const thirtyDaysAgo = new Date(midnightYesterday);
  thirtyDaysAgo.setDate(midnightYesterday.getDate() - 30);
  return { thirtyDaysAgo, midnightYesterday };
}

function filterOnDateRange({ created_at, thirtyDaysAgo, midnightYesterday }) {
  const createdAt = new Date(created_at);
  return createdAt >= thirtyDaysAgo && createdAt <= midnightYesterday;
}

/**
 * Parse CODEOWNERS: path -> list of individual owner logins (no org/team).
 */
function parseCodeowners(codeownersPath) {
  const content = fs.readFileSync(codeownersPath, 'utf8');
  const pathToOwners = {};
  for (const line of content.split('\n')) {
    const trimmed = line.trim();
    if (!trimmed || trimmed.startsWith('#')) continue;
    const parts = trimmed.split(/\s+/);
    if (parts.length < 2) continue;
    const pathPattern = parts[0].replace(/\/$/, '');
    const owners = parts.slice(1)
      .map((o) => o.replace(/^@/, ''))
      .filter((login) => !login.includes('/')); // only individuals
    if (owners.length > 0) {
      pathToOwners[pathPattern] = owners;
    }
  }
  return pathToOwners;
}

/**
 * Parse component_labels.txt: path (col1) -> label (col2). Returns also set of all component labels.
 */
function parseComponentLabels(labelsPath) {
  const content = fs.readFileSync(labelsPath, 'utf8');
  const pathToLabel = {};
  const allLabels = new Set();
  for (const line of content.split('\n')) {
    const trimmed = line.trim();
    if (!trimmed || trimmed.startsWith('#')) continue;
    const [compPath, label] = trimmed.split(/\s+/);
    if (compPath && label && label.length <= 50) {
      pathToLabel[compPath] = label;
      allLabels.add(label);
    }
  }
  return { pathToLabel, allLabels };
}

/**
 * Build label -> list of individual code owner logins.
 * CODEOWNERS can have path with trailing slash; component_labels has path without.
 */
function buildLabelToCodeOwners(pathToOwners, pathToLabel) {
  const labelToOwners = {};
  for (const [compPath, label] of Object.entries(pathToLabel)) {
    let owners = pathToOwners[compPath];
    if (!owners) {
      owners = pathToOwners[compPath + '/'] || [];
    }
    if (owners.length > 0) {
      labelToOwners[label] = owners;
    }
  }
  return labelToOwners;
}

async function searchIssuesAndPrs(octokit, query, thirtyDaysAgo, midnightYesterday) {
  const from = thirtyDaysAgo.toISOString().slice(0, 10);
  const to = midnightYesterday.toISOString().slice(0, 10);
  const q = `repo:${REPO_OWNER}/${REPO_NAME} ${query} created:${from}..${to}`;
  const items = [];
  let page = 1;
  const perPage = 100;
  while (true) {
    const { data } = await octokit.search.issuesAndPullRequests({
      q,
      per_page: perPage,
      page,
    });
    items.push(...data.items);
    if (data.items.length < perPage) break;
    page++;
  }
  return items;
}

async function getPrsInWindow(octokit, thirtyDaysAgo, midnightYesterday) {
  return searchIssuesAndPrs(octokit, 'is:pr', thirtyDaysAgo, midnightYesterday);
}

async function getIssuesInWindow(octokit, thirtyDaysAgo, midnightYesterday) {
  return searchIssuesAndPrs(octokit, 'is:issue', thirtyDaysAgo, midnightYesterday);
}

function getComponentLabelsOnItem(item, componentLabels) {
  const names = (item.labels || []).map((l) => l.name);
  return names.filter((name) => componentLabels.has(name));
}

async function getReviewAndCommentLogins(octokit, owner, repo, prNumber) {
  const logins = new Set();
  try {
    const { data: reviews } = await octokit.pulls.listReviews({
      owner,
      repo,
      pull_number: prNumber,
    });
    for (const r of reviews) {
      if (r.user && r.user.login) logins.add(r.user.login);
    }
    const { data: comments } = await octokit.issues.listComments({
      owner,
      repo,
      issue_number: prNumber,
    });
    for (const c of comments) {
      if (c.user && c.user.login) logins.add(c.user.login);
    }
  } catch (e) {
    debug({ msg: 'getReviewAndCommentLogins error', owner, repo, prNumber, error: e.message });
  }
  return logins;
}

async function getIssueCommentLogins(octokit, owner, repo, issueNumber) {
  const logins = new Set();
  try {
    const { data: comments } = await octokit.issues.listComments({
      owner,
      repo,
      issue_number: issueNumber,
    });
    for (const c of comments) {
      if (c.user && c.user.login) logins.add(c.user.login);
    }
  } catch (e) {
    debug({ msg: 'getIssueCommentLogins error', owner, repo, issueNumber, error: e.message });
  }
  return logins;
}

/**
 * Get per-label code owners: for each label in labelsOnItem, add (label, login) pairs
 * to the given map. Returns Map<login, Set<label>> for quick lookup.
 */
function getCodeOwnersByLabel(labelsOnItem, labelToOwners) {
  const ownerToLabels = new Map();
  for (const label of labelsOnItem) {
    const owners = labelToOwners[label];
    if (!owners) continue;
    for (const login of owners) {
      if (!ownerToLabels.has(login)) ownerToLabels.set(login, new Set());
      ownerToLabels.get(login).add(label);
    }
  }
  return ownerToLabels;
}

/**
 * Get owner and repo from a search result item (PR or issue). Fall back to upstream constants if missing.
 */
function getRepoFromItem(item) {
  const owner = item.repository?.owner?.login || item.base?.repo?.owner?.login || REPO_OWNER;
  const repo = item.repository?.name || item.base?.repo?.name || REPO_NAME;
  return { owner, repo };
}

/**
 * Stats aggregated by (code owner, component). Shape: byCodeOwner[login][componentLabel] = { total, responded }.
 */
async function computePrStats(octokit, prs, labelToOwners, componentLabels, thirtyDaysAgo, midnightYesterday) {
  const byCodeOwnerAndComponent = {};

  for (const pr of prs) {
    if (!filterOnDateRange({ created_at: pr.created_at, thirtyDaysAgo, midnightYesterday })) continue;
    const labelsOnPr = getComponentLabelsOnItem(pr, componentLabels);
    if (labelsOnPr.length === 0) continue;

    const ownerToLabels = getCodeOwnersByLabel(labelsOnPr, labelToOwners);
    if (ownerToLabels.size === 0) continue;

    const { owner, repo } = getRepoFromItem(pr);
    const respondents = await getReviewAndCommentLogins(octokit, owner, repo, pr.number);

    for (const [login, labels] of ownerToLabels) {
      for (const label of labels) {
        if (!byCodeOwnerAndComponent[login]) byCodeOwnerAndComponent[login] = {};
        if (!byCodeOwnerAndComponent[login][label]) {
          byCodeOwnerAndComponent[login][label] = { total: 0, responded: 0 };
        }
        byCodeOwnerAndComponent[login][label].total++;
        if (respondents.has(login)) {
          byCodeOwnerAndComponent[login][label].responded++;
        }
      }
    }
  }

  return byCodeOwnerAndComponent;
}

async function computeIssueStats(octokit, issues, labelToOwners, componentLabels, thirtyDaysAgo, midnightYesterday) {
  const byCodeOwnerAndComponent = {};

  for (const issue of issues) {
    if (!filterOnDateRange({ created_at: issue.created_at, thirtyDaysAgo, midnightYesterday })) continue;
    const labelsOnIssue = getComponentLabelsOnItem(issue, componentLabels);
    if (labelsOnIssue.length === 0) continue;

    const ownerToLabels = getCodeOwnersByLabel(labelsOnIssue, labelToOwners);
    if (ownerToLabels.size === 0) continue;

    const { owner, repo } = getRepoFromItem(issue);
    const respondents = await getIssueCommentLogins(octokit, owner, repo, issue.number);

    for (const [login, labels] of ownerToLabels) {
      for (const label of labels) {
        if (!byCodeOwnerAndComponent[login]) byCodeOwnerAndComponent[login] = {};
        if (!byCodeOwnerAndComponent[login][label]) {
          byCodeOwnerAndComponent[login][label] = { total: 0, responded: 0 };
        }
        byCodeOwnerAndComponent[login][label].total++;
        if (respondents.has(login)) {
          byCodeOwnerAndComponent[login][label].responded++;
        }
      }
    }
  }

  return byCodeOwnerAndComponent;
}

/**
 * Flatten byCodeOwnerAndComponent into rows: one row per (code owner, component).
 */
function formatTable(byCodeOwnerAndComponent, kind) {
  const rows = [];
  for (const [login, byComponent] of Object.entries(byCodeOwnerAndComponent)) {
    for (const [component, v] of Object.entries(byComponent)) {
      if (v.total === 0) continue;
      const pct = Math.round((100 * v.responded) / v.total);
      const meets = pct >= RESPONSE_THRESHOLD_PCT ? 'Yes' : 'No';
      rows.push([`@${login}`, component, String(v.total), String(v.responded), `${pct}%`, meets]);
    }
  }
  rows.sort((a, b) => {
    const cmpLogin = a[0].localeCompare(b[0]);
    if (cmpLogin !== 0) return cmpLogin;
    return a[1].localeCompare(b[1]);
  });

  if (rows.length === 0) {
    return `No ${kind} with component labels in this period.\n`;
  }

  const header = '| Code owner | Component | Total | Responded | % | Meets 80%? |';
  const sep = '| --- | --- | --- | --- | --- | --- |';
  const body = rows.map((r) => `| ${r.join(' | ')} |`).join('\n');
  return `${header}\n${sep}\n${body}\n`;
}

/**
 * Same as formatTable but only rows where the code owner did not meet the threshold.
 */
function formatTableBelowThreshold(byCodeOwnerAndComponent, kind) {
  const rows = [];
  for (const [login, byComponent] of Object.entries(byCodeOwnerAndComponent)) {
    for (const [component, v] of Object.entries(byComponent)) {
      if (v.total === 0) continue;
      const pct = Math.round((100 * v.responded) / v.total);
      if (pct >= RESPONSE_THRESHOLD_PCT) continue;
      const meets = 'No';
      rows.push([`@${login}`, component, String(v.total), String(v.responded), `${pct}%`, meets]);
    }
  }
  rows.sort((a, b) => {
    const cmpLogin = a[0].localeCompare(b[0]);
    if (cmpLogin !== 0) return cmpLogin;
    return a[1].localeCompare(b[1]);
  });

  if (rows.length === 0) {
    return `All code owners meet the threshold for ${kind} in this period.\n`;
  }

  const header = '| Code owner | Component | Total | Responded | % | Meets 80%? |';
  const sep = '| --- | --- | --- | --- | --- | --- |';
  const body = rows.map((r) => `| ${r.join(' | ')} |`).join('\n');
  return `${header}\n${sep}\n${body}\n`;
}

function generateReport(prStats, issueStats, lookbackData) {
  const out = [
    `## Code owner activity report`,
    ``,
    `Period: ${lookbackData.thirtyDaysAgo.toISOString().slice(0, 10)} – ${lookbackData.midnightYesterday.toISOString().slice(0, 10)}`,
    ``,
    `Each code owner (individuals only) must have reviewed or replied to at least **${RESPONSE_THRESHOLD_PCT}%** of the PRs and issues where they were the code owner for the component.`,
    ``,
    `### Pull requests`,
    ``,
    formatTable(prStats, 'PRs'),
    `### PRs below threshold`,
    ``,
    formatTableBelowThreshold(prStats, 'PRs'),
    `### Issues`,
    ``,
    formatTable(issueStats, 'issues'),
    `### Issues below threshold`,
    ``,
    formatTableBelowThreshold(issueStats, 'issues'),
  ];
  return out.join('\n');
}

async function createIssue(octokit, context, report, lookbackData) {
  const title = `Code owner activity: ${lookbackData.thirtyDaysAgo.toISOString().slice(0, 10)} – ${lookbackData.midnightYesterday.toISOString().slice(0, 10)}`;
  return octokit.issues.create({
    owner: context.payload.repository.owner.login,
    repo: REPO_NAME,
    title,
    body: report,
    labels: ['report'],
  });
}

async function main({ github, context }) {
  debug({ msg: 'generate-codeowners-activity running' });
  const octokit = github.rest;
  const lookbackData = genLookbackDates();

  const codeownersPath = path.join(process.cwd(), '.github', 'CODEOWNERS');
  const labelsPath = path.join(process.cwd(), '.github', 'component_labels.txt');
  if (!fs.existsSync(codeownersPath) || !fs.existsSync(labelsPath)) {
    throw new Error('CODEOWNERS or component_labels.txt not found');
  }

  const pathToOwners = parseCodeowners(codeownersPath);
  const { pathToLabel, allLabels: componentLabels } = parseComponentLabels(labelsPath);
  const labelToOwners = buildLabelToCodeOwners(pathToOwners, pathToLabel);

  const [prs, issues] = await Promise.all([
    getPrsInWindow(octokit, lookbackData.thirtyDaysAgo, lookbackData.midnightYesterday),
    getIssuesInWindow(octokit, lookbackData.thirtyDaysAgo, lookbackData.midnightYesterday),
  ]);

  // Log which repo the search returned (first PR) so we can confirm token can access it
  if (prs.length > 0) {
    const { owner, repo } = getRepoFromItem(prs[0]);
    debug({ msg: 'Fetching reviews/comments for repo', owner, repo, prCount: prs.length, issueCount: issues.length });
  }

  const [prStats, issueStats] = await Promise.all([
    computePrStats(octokit, prs, labelToOwners, componentLabels, lookbackData.thirtyDaysAgo, lookbackData.midnightYesterday),
    computeIssueStats(octokit, issues, labelToOwners, componentLabels, lookbackData.thirtyDaysAgo, lookbackData.midnightYesterday),
  ]);

  const report = generateReport(prStats, issueStats, lookbackData);

  // Dry run: set DRY_RUN=true or DRY_RUN=1 to print the report without creating an issue.
  const dryRun = process.env.DRY_RUN === 'true' || process.env.DRY_RUN === '1';
  if (dryRun) {
    debug({ msg: 'Dry run: skipping issue creation' });
    console.log('\n' + report);
    return;
  }

  await createIssue(octokit, context, report, lookbackData);
}

module.exports = async ({ github, context }) => {
  await main({ github, context });
};
