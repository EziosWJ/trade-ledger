# Issue tracker: GitHub

Issues and specs for this repo live as GitHub issues. Use the `gh` CLI for all operations.
Infer the repo from `git remote -v`; `gh` does this automatically inside a clone.

## Conventions

- Create: `gh issue create --title "..." --body "..."`
- Read: `gh issue view <number> --comments`; also fetch labels.
- List: `gh issue list --state open --json number,title,body,labels,comments`, with appropriate label and state filters.
- Comment: `gh issue comment <number> --body "..."`
- Apply/remove labels: `gh issue edit <number> --add-label "..."` / `--remove-label "..."`
- Close: `gh issue close <number> --comment "..."`

When a skill says "publish to the issue tracker", create a GitHub issue.
When a skill says "fetch the relevant ticket", run `gh issue view <number> --comments`.

## Pull requests as a triage surface

**PRs as a request surface: no.**

When enabled, use `gh pr view <number> --comments` and `gh pr diff <number>` to read PRs.
List open PRs and retain external authors with authorAssociation of
CONTRIBUTOR, FIRST_TIME_CONTRIBUTOR, or NONE.
Use `gh pr comment`, `gh pr edit`, and `gh pr close` for updates.

GitHub shares one number space across issues and PRs. Resolve ambiguous
references with `gh pr view <number>`, falling back to `gh issue view <number>`.

## Wayfinding operations

Used by `/wayfinder`. The map is one issue with child issues as tickets.

- Map: label `wayfinder:map`; body contains Notes, Decisions-so-far, and Fog.
- Child: link as a GitHub sub-issue via `gh api`. If unavailable, add a task
  list entry to the map and `Part of #<map>` to the child.
  Use `wayfinder:<type>` labels: research, prototype, grilling, or task.
- Blocking: use native issue dependencies.
  Add an edge with
  `gh api --method POST repos/<owner>/<repo>/issues/<child>/dependencies/blocked_by -F issue_id=<blocker-db-id>`.
  Get the database ID with
  `gh api repos/<owner>/<repo>/issues/<number> --jq .id`.
  Use the database ID, not the issue number or node_id.
  If dependencies are unavailable, use a `Blocked by: #<n>, #<n>` line.
- Frontier: list open children scoped to the map. Exclude assigned tickets
  and tickets with open blockers. Use
  `issue_dependencies_summary.blocked_by`, or check the fallback references.
  Choose the first eligible child in map order.
- Claim: `gh issue edit <number> --add-assignee @me`, the session's first write.
- Resolve: comment with the answer, close the child, then append a gist and
  link to the map's Decisions-so-far.
