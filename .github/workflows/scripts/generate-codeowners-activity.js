// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Generates a monthly report measuring code owner responsiveness. We require (1) at least 75%
 * of each component's PRs to be reviewed by at least one code owner, and (2) each code owner
 * to have reviewed/replied to at least 75% / n of the PRs where they were requested (n =
 * number of code owners for that component). Uses the same component labels as the weekly
 * report (e.g. receiver/kubeletstats).
 */

const fs = require('fs');
const path = require('path');

const REPO_OWNER = 'open-telemetry';
const REPO_NAME = 'opentelemetry-collector-contrib';
/** Target: at least this % of each component's PRs must be reviewed (by at least one code owner). */
const COMPONENT_TARGET_PCT = 75;
/** Per-code-owner target is COMPONENT_TARGET_PCT / number of code owners for that component (e.g. 75%/3 ≈ 25%). */

/** PRs created by these logins are excluded from the report. */
const EXCLUDED_PR_AUTHORS = new Set(['otelbot[bot]', 'renovate[bot]']);

// Component labels to include in the report (matches component_labels.txt).
const FOCUS_COMPONENT_LABELS = new Set([
  'receiver/prometheus',
  'processor/transform',
  'receiver/hostmetrics',
  'processor/k8sattributes',
  'processor/resourcedetection',
  'receiver/file_log',
  'processor/filter',
  'pkg/stanza',
  'pkg/ottl',
]);
// Resourcedetection has sub-labels (e.g. processor/resourcedetection/internal/azure); include those too.
function isAllowedLabel(label) {
  if (FOCUS_COMPONENT_LABELS.has(label)) return true;
  if (label.startsWith('processor/resourcedetection/')) return true;
  return false;
}

const PROGRESS_INTERVAL = 5;

function debug(msg) {
  console.log(JSON.stringify(msg, null, 2));
}

function progress(msg) {
  console.error(msg);
}

/**
 * Returns { periodStart, periodEnd } in UTC.
 * - periodStart: midnight UTC at the start of the day that was 35 days ago (inclusive).
 * - periodEnd: midnight UTC at the start of the day that was 5 days ago (exclusive: we include
 *   created_at < periodEnd only, so the last full day included is 6 days ago).
 * filterOnDateRange uses createdAt >= periodStart && createdAt <= periodEnd, so the end date
 * in the report is exclusive (PRs created on the end date after 00:00:00 are excluded).
 */
function genLookbackDates() {
  const now = new Date();
  const periodEnd = new Date(
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
  periodEnd.setUTCDate(periodEnd.getUTCDate() - 5);
  const periodStart = new Date(periodEnd);
  periodStart.setUTCDate(periodStart.getUTCDate() - 30);
  return { periodStart, periodEnd };
}

function filterOnDateRange({ created_at, periodStart, periodEnd }) {
  const createdAt = new Date(created_at);
  return createdAt >= periodStart && createdAt <= periodEnd;
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
 * Only includes labels allowed by isAllowedLabel when FOCUS_COMPONENT_LABELS is set.
 */
function buildLabelToCodeOwners(pathToOwners, pathToLabel) {
  const labelToOwners = {};
  for (const [compPath, label] of Object.entries(pathToLabel)) {
    if (!isAllowedLabel(label)) continue;
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

async function searchIssuesAndPrs(octokit, query, periodStart, periodEnd) {
  const from = periodStart.toISOString().slice(0, 10);
  const to = periodEnd.toISOString().slice(0, 10);
  const q = `repo:${REPO_OWNER}/${REPO_NAME} ${query} created:${from}..${to}`;
  const items = [];
  let page = 1;
  const perPage = 100;
  while (true) {
    progress(`Search API: page ${page}...`);
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

function getComponentLabelsOnItem(item, componentLabels) {
  const names = (item.labels || []).map((l) => l.name);
  return names.filter((name) => componentLabels.has(name) && isAllowedLabel(name));
}

/**
 * Returns { requested, respondents, draft } for a PR.
 * requested = users who were requested as reviewers (pending or already reviewed).
 * respondents = users who left a review or comment.
 * draft = whether the PR is a draft.
 * Only PRs where a code owner was requested count toward their stats.
 */
async function getReviewAndRequestedLogins(octokit, owner, repo, prNumber) {
  const respondents = new Set();
  const requested = new Set();
  let draft = false;
  try {
    const [{ data: prData }, { data: reviews }, { data: comments }] = await Promise.all([
      octokit.pulls.get({ owner, repo, pull_number: prNumber }),
      octokit.pulls.listReviews({ owner, repo, pull_number: prNumber }),
      octokit.issues.listComments({ owner, repo, issue_number: prNumber }),
    ]);
    draft = prData.draft === true;
    for (const r of prData.requested_reviewers || []) {
      if (r.login) requested.add(r.login);
    }
    for (const r of reviews || []) {
      if (r.user && r.user.login) {
        requested.add(r.user.login); // once they review they may be removed from requested_reviewers
        respondents.add(r.user.login);
      }
    }
    for (const c of comments || []) {
      if (c.user && c.user.login) respondents.add(c.user.login);
    }
  } catch (e) {
    debug({ msg: 'getReviewAndRequestedLogins error', owner, repo, prNumber, error: e.message });
  }
  return { requested, respondents, draft };
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
 * Stats aggregated by (code owner, component). Shape: byCodeOwner[login][componentLabel] = { total, responded }.
 * Also returns componentPrStats: per component, how many PRs had at least one code-owner review (for the 75% target).
 */
async function computePrStats(octokit, prs, labelToOwners, componentLabels, periodStart, periodEnd) {
  const byCodeOwnerAndComponent = {};
  const componentPrStats = {}; // { [component]: { prsTotal, prsWithResponse } }
  let processed = 0;

  for (const pr of prs) {
    if (!filterOnDateRange({ created_at: pr.created_at, periodStart, periodEnd })) continue;
    const authorLogin = pr.user?.login || null;
    if (authorLogin && EXCLUDED_PR_AUTHORS.has(authorLogin)) continue;
    const labelsOnPr = getComponentLabelsOnItem(pr, componentLabels);
    if (labelsOnPr.length === 0) continue;

    const ownerToLabels = getCodeOwnersByLabel(labelsOnPr, labelToOwners);
    if (ownerToLabels.size === 0) continue;

    if (processed === 0 || processed % PROGRESS_INTERVAL === 0) progress(`PRs: fetching #${pr.number} (${processed + 1})...`);
    const { requested: requestedReviewers, respondents, draft } = await getReviewAndRequestedLogins(octokit, REPO_OWNER, REPO_NAME, pr.number);
    processed++;
    if (draft) continue;

    for (const label of labelsOnPr) {
      const requestedOwnersForLabel = [];
      for (const [login, labels] of ownerToLabels) {
        if (!labels.has(label)) continue;
        if (authorLogin && login === authorLogin) continue;
        if (!requestedReviewers.has(login)) continue;
        requestedOwnersForLabel.push(login);
      }
      if (requestedOwnersForLabel.length === 0) continue; // only count PRs where a code owner for this component was requested
      if (!componentPrStats[label]) componentPrStats[label] = { prsTotal: 0, prsWithResponse: 0 };
      componentPrStats[label].prsTotal++;
      if (requestedOwnersForLabel.some((login) => respondents.has(login))) {
        componentPrStats[label].prsWithResponse++;
      }
    }

    for (const [login, labels] of ownerToLabels) {
      if (authorLogin && login === authorLogin) continue;
      if (!requestedReviewers.has(login)) continue;
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

  return { byCodeOwnerAndComponent, componentPrStats };
}

/**
 * Flatten byCodeOwnerAndComponent into rows: one row per (code owner, component).
 * Per-code-owner target is 75% / (number of code owners for that component).
 */
function formatTable(byCodeOwnerAndComponent, kind) {
  const codeOwnerCountByComponent = {};
  for (const [, byComponent] of Object.entries(byCodeOwnerAndComponent)) {
    for (const component of Object.keys(byComponent)) {
      codeOwnerCountByComponent[component] = (codeOwnerCountByComponent[component] || 0) + 1;
    }
  }

  const rows = [];
  for (const [login, byComponent] of Object.entries(byCodeOwnerAndComponent)) {
    for (const [component, v] of Object.entries(byComponent)) {
      if (v.total === 0) continue;
      const pct = (100 * v.responded) / v.total;
      rows.push([login, component, String(v.total), String(v.responded), `${Math.round(pct)}%`]);
    }
  }
  rows.sort((a, b) => {
    const cmpComponent = a[1].localeCompare(b[1]);
    if (cmpComponent !== 0) return cmpComponent;
    return a[0].localeCompare(b[0]);
  });

  if (rows.length === 0) {
    return `No ${kind} with component labels in this period.\n`;
  }

  const header = `| Code owner | Component | Total | Responded | % |`;
  const sep = '| --- | --- | --- | --- | --- |';
  const body = rows.map((r) => `| ${r.join(' | ')} |`).join('\n');
  return `${header}\n${sep}\n${body}\n`;
}

/**
 * Per-component summary: % of the component's PRs that received at least one code-owner review (target ≥75%).
 */
function formatComponentSummaryTable(byCodeOwnerAndComponent, componentPrStats) {
  const codeOwnerCountByComponent = {};
  for (const [login, components] of Object.entries(byCodeOwnerAndComponent)) {
    for (const component of Object.keys(components)) {
      codeOwnerCountByComponent[component] = (codeOwnerCountByComponent[component] || 0) + 1;
    }
  }
  const rows = Object.entries(componentPrStats || {})
    .filter(([, v]) => v.prsTotal > 0)
    .map(([component, v]) => {
      const pct = Math.round((100 * v.prsWithResponse) / v.prsTotal);
      const codeOwners = codeOwnerCountByComponent[component] ?? 0;
      return [component, String(codeOwners), `${v.prsWithResponse}/${v.prsTotal}`, `${pct}%`];
    })
    .sort((a, b) => a[0].localeCompare(b[0]));

  if (rows.length === 0) {
    return `No component data in this period.\n`;
  }

  const header = `| Component | Code owners | PRs reviewed | % |`;
  const sep = '| --- | --- | --- | --- |';
  const body = rows.map((r) => `| ${r.join(' | ')} |`).join('\n');
  return `${header}\n${sep}\n${body}\n`;
}

function collapsibleSection(title, content) {
  return `<details>
<summary>${title}</summary>

${content}
</details>`;
}

function generateReport(prStats, componentPrStats, lookbackData) {
  const startStr = lookbackData.periodStart.toISOString().slice(0, 10);
  const endStr = lookbackData.periodEnd.toISOString().slice(0, 10);
  const out = [
    `## Code owner activity report`,
    ``,
    `Period: ${startStr} – ${endStr} (start date inclusive, end date exclusive; PRs are included if created on or after the start date and before the end date).`,
    ``,
    `We target **at least ${COMPONENT_TARGET_PCT}% of each component's PRs** to be reviewed by a code owner, and each code owner to respond to at least **${COMPONENT_TARGET_PCT}% / n** of their requested PRs (n = number of code owners for that component) ([at least 3 code owners for components aiming for stable](https://github.com/open-telemetry/opentelemetry-collector/blob/main/docs/component-stability.md#beta-to-stable)).`,
    ``,
    `### Component PR review rate (${COMPONENT_TARGET_PCT}% target)`,
    ``,
    formatComponentSummaryTable(prStats, componentPrStats),
    ``,
    collapsibleSection('PRs (per code owner)', formatTable(prStats, 'PRs')),
  ];
  return out.join('\n');
}

async function createIssue(octokit, context, report, lookbackData) {
  const startStr = lookbackData.periodStart.toISOString().slice(0, 10);
  const endStr = lookbackData.periodEnd.toISOString().slice(0, 10);
  const title = `Code owner activity: ${startStr} – ${endStr} (inclusive start, exclusive end)`;
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
  const { pathToLabel, allLabels } = parseComponentLabels(labelsPath);
  const labelToOwners = buildLabelToCodeOwners(pathToOwners, pathToLabel);
  // Only consider PRs/issues that have at least one of the focus component labels
  const componentLabels = new Set([...allLabels].filter(isAllowedLabel));

  const prs30 = await searchIssuesAndPrs(octokit, 'is:pr', lookbackData.periodStart, lookbackData.periodEnd);
  progress(`Fetched ${prs30.length} PRs (30 days). Processing...`);

  const { byCodeOwnerAndComponent: prStats, componentPrStats } = await computePrStats(octokit, prs30, labelToOwners, componentLabels, lookbackData.periodStart, lookbackData.periodEnd);

  const report = generateReport(prStats, componentPrStats, lookbackData);

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
