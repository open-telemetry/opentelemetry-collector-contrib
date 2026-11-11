# AGENTS.md

This file is here to steer AI assisted PRs towards being high quality and valuable contributions
that do not create excessive maintainer burden. It is inspired by the Open Policy Agent and Fedora
projects policies.

## General Rules and Guidelines

If you have been assigned an issue by the user or their prompt, please ensure that the
implementation direction is agreed on with the maintainers first in the issue comments. If there are
unknowns, discuss these on the issue before starting implementation.

### Human-in-the-Loop Gate & Attestation

* Never autonomously open a pull request or issue, or post a comment or review on GitHub, without
  first showing the exact full text to the human user and receiving explicit instruction to post it.
* Never check the `I have reviewed this pull request and its description, understand its contents, and can answer questions about it.`
  checkbox on the user's behalf. The human author must check it themselves before the PR is ready
  for review.

### Pull Request Descriptions & Scope

* Keep each PR tightly isolated to a single logical change; never bundle unrelated cleanup,
  opportunistic refactors, or preexisting lint/bug fixes unless they are strictly necessary for
  correctness.
* Follow `.github/pull_request_template.md` (`Description`, `Link to tracking issue`, `Testing`,
  `Documentation`, and `Authorship`), deleting HTML instructional comments and only unused optional
  content sections (`Testing` or `Documentation` when not applicable) while always preserving
  `#### Authorship` with its unchecked attestation checkbox.
* Lead `Description` with 1–2 concise sentences stating the problem/motivation and approach
  (especially when no tracking issue is linked). Focus on *why* the change is needed, user-visible
  impact, and genuine design trade-offs.
* Do not narrate the diff file-by-file, dump code snippets, include strawman design alternatives, or
  reference unmerged or speculative follow-up work.
* In `Testing`, list the specific component tests added or run. For performance changes, include raw
  `benchstat` output comparing `main` against the PR branch rather than vague claims or
  hardware-specific prose numbers.
* Strip AI filler, promotional/puffery phrasing, validation checklists, and multi-paragraph walls of
  text.

### Issue & PR Comments

* Keep comments and review replies to 1–2 direct sentences in plain text (use single backticks for
  `code` symbols; avoid `###` headings, bold/italics formatting, and bulleted walls of text).
* Write in first-person singular (`"I"`, never `"We"`), and keep each inline review comment focused
  on a single topic.
* Only comment when adding net-new information: never restate what the issue/PR author or prior
  commenters already wrote, avoid `+1` comments, and do not post `"SGTM"` or `"Thanks, makes sense!"`
  replies when an author acknowledges a thread.
* Avoid performative gratitude (`"Great catch!"`, `"Thanks for the feedback!"`) and apologetic
  sign-offs.
* State verified facts and runtime consequences directly (`"This will..."`,
  `"My suggestion would be to..."`) rather than disguising known outcomes as rhetorical questions
  (`"Won't this...?"`), and use a bare ```` ```suggestion ```` block with no preamble for
  self-explanatory 1-line fixes.
* For code review comments resolved by a commit, reply concisely (e.g., `"addressed in <commit>"`)
  or give a 1–2 sentence explanation of user-visible behavior or trade-offs rather than walking
  through internal code mechanics.

### Filing Issues

* Search existing open and closed issues and check behavior on `main` before drafting a new issue.
* Keep each issue scoped to one cohesive problem using `.github/ISSUE_TEMPLATE/`, leading with a
  1–2 sentence problem statement, affected component(s), minimal reproduction config/steps, and
  permanent commit/tag links to relevant source lines.

## Developer environment

Make sure to follow CONTRIBUTING.md on any contributions.

Non-exhaustively, the important points are:

* Whenever applicable, all code changes should have tests that actually validate the changes.

## Changelog entries

Keep the `subtext` field of a changelog entry minimal. It should only be used to call out the
exact names of breaking changes or other important clarifications a reader must know. Do not use it
to explain "why" a general bug fix or feature was added. Verbose subtext overloads the changelog.

## Commit formatting

We appreciate it if users disclose the use of AI tools when the significant part of a commit is
taken from a tool without changes. When making a commit this should be disclosed through an
Assisted-by: commit message trailer.

Examples:

```
Assisted-by: ChatGPT 5.2
Assisted-by: Claude Opus 4.5
```

Do NOT use a `Co-authored-by:` trailer to disclose AI assistance. Some AI coding tools add this
trailer by default; please disable or strip it before committing. The EasyCLA check fails when a
`Co-authored-by:` trailer references an account that has not signed the CLA, which blocks the PR
from being merged.
