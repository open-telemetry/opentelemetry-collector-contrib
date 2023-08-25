const fs = require('fs');
const path = require('path');
const yaml = require('js-yaml');


function debug(msg) {
  console.log(JSON.stringify(msg, null, 2))
}

async function getIssues(github, queryParams, filterPrs = true) {
  let allIssues = [];
  try {
    while (true) {
      const response = await github.issues.listForRepo(queryParams);
      // filter out pull requests
      const issues = filterPrs ? response.data.filter(issue => !issue.pull_request) : response.data;
      allIssues = allIssues.concat(issues);

      // Check the 'link' header to see if there are more pages
      const linkHeader = response.headers.link;
      if (!linkHeader || !linkHeader.includes('rel="next"')) {
        break;
      }

      queryParams.page++;
    }
    return allIssues;
  } catch (error) {
    console.error('Error fetching issues:', error);
    return [];
  }
}

function genLookbackDates() {
  const now = new Date();
  const midnightYesterday = new Date(now.getFullYear(), now.getMonth(), now.getDate() - 1);
  const sevenDaysAgo = new Date(midnightYesterday);
  sevenDaysAgo.setDate(midnightYesterday.getDate() - 7);
  return { sevenDaysAgo, midnightYesterday };
}

function filterOnDateRange({ issue, sevenDaysAgo, midnightYesterday }) {
  const createdAt = new Date(issue.created_at);
  return createdAt >= sevenDaysAgo && createdAt <= midnightYesterday;
}

async function getNewIssues({github, context}) {
  const { sevenDaysAgo, midnightYesterday } = genLookbackDates();
  const queryParams = {
    owner: 'open-telemetry',
    repo: 'opentelemetry-collector-contrib',
    state: 'all', // To get both open and closed issues
    per_page: 100, // Number of items per page (maximum allowed)
    page: 1, // Start with page 1
    since: sevenDaysAgo.toISOString(),
  };

  try {
    const allIssues = await getIssues(github, queryParams)
    const filteredIssues = allIssues.filter(issue => filterOnDateRange({ issue, sevenDaysAgo, midnightYesterday }));
    return filteredIssues;
  } catch (error) {
    console.error('Error fetching issues:', error);
    return [];
  }
}

async function getTargetLabelIssues({octokit, labels, filterPrs, context}) {
  const queryParams = {
    owner: context.repository_owner,
    repo: 'opentelemetry-collector-contrib',
    state: 'open',
    per_page: 100, // Number of items per page (maximum allowed)
    page: 1, // Start with page 1
    labels
  };
  debug({msg: "fetching issues", queryParams})
  try {
    const allIssues = await getIssues(octokit, queryParams, filterPrs)
    return allIssues;
  } catch (error) {
    console.error('Error fetching issues:', error);
    return [];
  }
}

async function getIssuesData({github, context}) {
  const targetLabels = {
    "needs triage": {
      filterPrs: true,
      alias: "issuesTriage",
    },
    "ready to merge": {
      filterPrs: false,
      alias: "issuesReadyToMerge",
    },
    "Sponsor Needed": {
      filterPrs: true,
      alias: "issuesSponsorNeeded",
    },
  };

  const issuesNew = await getNewIssues({github: github.rest, context});
  const issuesWithLabels = {};
  for (const lbl of Object.keys(targetLabels)) {
    const filterPrs = targetLabels[lbl].filterPrs;
    const resp = await getTargetLabelIssues({octokit: github.rest, labels: lbl, filterPrs, context});
    issuesWithLabels[lbl] = resp;
  }

  // tally results
  const stats = {
    issuesNew: {
      title: "New issues",
      count: 0,
      data: []
    },
    issuesTriage: {
      title: "Issues needing triage",
      count: 0,
      data: []
    },
    issuesReadyToMerge: {
      title: "Issues ready to merge",
      count: 0,
      data: []
    },
    issuesSponsorNeeded: {
      title: "Issues needing sponsorship",
      count: 0,
      data: []
    },
    issuesNewSponsorNeeded: {
      title: "New issues needing sponsorship",
      count: 0,
      data: []
    },
  }

  // add new issues
  issuesNew.forEach(issue => {
    stats.issuesNew.count++;
    const { url, title, number } = issue;
    stats.issuesNew.data.push({ url, title, number });
  });

  // add issues with labels
  for (const lbl of Object.keys(targetLabels)) {
    const alias = targetLabels[lbl].alias;
    stats[alias].count = issuesWithLabels[lbl].length;
    stats[alias].data = issuesWithLabels[lbl].map(issue => {
      const { url, title, number } = issue;
      return { url, title, number };
    })
  }

  // add new issues with sponsor needed label
  const { sevenDaysAgo, midnightYesterday } = genLookbackDates();
  const sponsorNeededIssues = issuesWithLabels["Sponsor Needed"].filter(issue => filterOnDateRange({ issue, sevenDaysAgo, midnightYesterday }));
  sponsorNeededIssues.forEach(issue => {
    stats.issuesNewSponsorNeeded.count++;
    const { url, title, number } = issue;
    stats.issuesNewSponsorNeeded.data.push({ url, title, number });
  });
  return stats
}

function generateReport({ issuesData, previousReport, componentData }) {
  const out = [
    `## Format`,
    "- `{CATEGORY}: {COUNT} ({CHANGE_FROM_PREVIOUS_WEEK})`",
    "## Issues Report",
    '<ul>'];

  // generate report for issues
  for (const lbl of Object.keys(issuesData)) {
    const section = [``];
    const { count, data, title } = issuesData[lbl];

    if (previousReport === null) {
      section.push(`<li> ${title}: ${count}`);
    } else {
      const previousCount = previousReport.issuesData[lbl].count; 
      section.push(`<li> ${title}: ${count} (${count - previousCount})`);
    }

    // generate summary if issues exist
    if (data.length !== 0) {
      section.push(`<details>
    <summary> Issues </summary>
    <ul>
    ${data.map((issue) => {
        const { url, title, number } = issue;
        return `<li><a href="${url}">#${number}</a> ${title}</li>`;
      }).join('\n')}
    </ul>
</details>`);
    }

    section.push('</li>');
    out.push(section.join('\n'));
  }
  out.push('</ul>');

  // generate report for components
  out.push('\n## Components Report', '<ul>');
  for (const lbl of Object.keys(componentData)) {
    const section = [``];
    const data = componentData[lbl];
    const count = Object.keys(data).length;

    section.push(`<li> ${lbl}: ${count}`);
    if (data.length !== 0) {
      section.push(`<details>
    <summary> Components </summary>
    <ul>
    ${Object.keys(data).map((compName) => {
        const {stability} = data[compName]
        return `<li>${compName}: ${JSON.stringify(stability)}}</li>`;
      }).join('\n')}
    </ul>
</details>`);
    }
    section.push('</li>');
    out.push(section.join('\n'));
  }
  out.push('</ul>');

  // add json data
  out.push('\n ## JSON Data');
  out.push('<!-- MACHINE GENERATED: DO NOT EDIT -->');
  out.push(`<details>
<summary>Expand</summary>
<pre>
{
  "issuesData": ${JSON.stringify(issuesData, null, 2)},
  "componentData": ${JSON.stringify(componentData, null, 2)}
}
</pre>
</details>`);
  const report = out.join('\n');
  return report;
}

async function createIssue({ github, lookbackData, report, context }) {
  const title = `Weekly Report: ${lookbackData.sevenDaysAgo.toISOString().slice(0, 10)} - ${lookbackData.midnightYesterday.toISOString().slice(0, 10)}`;
  return github.rest.issues.create({
    owner: context.repository_owner,
    repo: "opentelemetry-collector-contrib",
    title,
    body: report,
    labels: ["report"]
  })
}

async function getLastWeeksReport({ github, since, context }) {
  const issues = await github.rest.issues.listForRepo({
    owner: context.repository_owner,
    repo: 'opentelemetry-collector-contrib',
    state: 'all', // To get both open and closed issues
    labels: ["report"],
    since: since.toISOString(),
    per_page: 1,
    direction: "asc"
  });
  if (issues.data.length === 0) {
    return null;
  } 
  // grab earliest issue if multiple
  return issues.data[0];
}

function parseJsonFromText(text) {
  // Use regex to find the JSON data within the <pre></pre> tags
  const regex = /<pre>\s*([\s\S]*?)\s*<\/pre>/;
  const match = text.match(regex);

  if (match && match[1]) {
    // Parse the found string to JSON
    return JSON.parse(match[1]);
  } else {
    throw new Error("JSON data not found");
  }
}

async function processIssues({ github, context, lookbackData }) {
  const issuesData = await getIssuesData({github, context});

  const prevReportLookback = new Date(lookbackData.sevenDaysAgo)
  prevReportLookback.setDate(prevReportLookback.getDate() - 7)
  const previousReportIssue = await getLastWeeksReport({github, since: prevReportLookback, context});
  // initialize to zeros
  let previousReport = null;

  if (previousReportIssue !== null) {
    const {created_at, id, url, title} = previousReportIssue;
    debug({ msg: "previous issue", created_at, id, url, title })
    previousReport = parseJsonFromText(previousReportIssue.body)
  }

  return {issuesData, previousReport}


}

const findFilesByName = (startPath, filter) => {
  let results = [];

  // Check if directory exists
  let files;
  try {
      files = fs.readdirSync(startPath);
  } catch (error) {
      console.error("Error reading directory: ", startPath, error);
      return [];
  }

  for (let i = 0; i < files.length; i++) {
      const filename = path.join(startPath, files[i]);
      let stat;
      try {
          stat = fs.lstatSync(filename);
      } catch (error) {
          console.error("Error stating file: ", filename, error);
          continue;
      }
      
      if (stat.isDirectory()) {
          const innerResults = findFilesByName(filename, filter); // Recursive call
          results = results.concat(innerResults);
      } else if (filename.indexOf(filter) >= 0) {
          results.push(filename);
      }
  }
  return results;
};

function processFiles(files) {
  const results = {};

  for (let filePath of files) {
      const component = path.basename(path.dirname(path.dirname(filePath)));  // Top level directory
      const name = path.basename(path.dirname(filePath));                      // Directory of the file
      const fileData = fs.readFileSync(filePath, 'utf8');                   // Read the file as a string

      let data;
      try {
          data = yaml.load(fileData);  // Parse YAML
      } catch (err) {
          console.error(`Error parsing YAML for file ${filePath}:`, err);
          continue;  // Skip this file if there's an error in parsing
      }

      if (!results[component]) {
          results[component] = {};
      }

      results[component][name] = {
          path: filePath,
          data
      };
  }

  return results;
}

const processStatusResults = (results) => {
  const filteredResults = {};

  for (const component in results) {
      for (const name in results[component]) {
          const { path, data } = results[component][name];

          if (data && data.status && data.status.stability) {
              const { stability } = data.status;
              const statuses = ['unmaintained', 'deprecated'];

              for (const status of statuses) {
                  if (stability[status] && stability[status].length > 0) {
                      if (!filteredResults[status]) {
                          filteredResults[status] = {};
                      }
                      filteredResults[status][name] = { path, stability: data.status.stability, component };
                  }
              }
          }
      }
  }

  return filteredResults;
};

async function processComponents() {
  const results = findFilesByName(`.`, 'metadata.yaml');
  const resultsClean = processFiles(results)
  const resultsWithStability = processStatusResults(resultsClean)
  return resultsWithStability

}

async function main({ github, context }) {
  const lookbackData = genLookbackDates();
  const {issuesData, previousReport} = await processIssues({ github, context, lookbackData })
  const componentData = await processComponents()

  const report = generateReport({ issuesData, previousReport, componentData })

  await createIssue({github, lookbackData, report, context});
}

module.exports = async ({ github, context }) => {
  await main({ github, context })
}