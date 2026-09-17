# Domain Docs

## Layout: single-context

- `CONTEXT.md` at the repo root: domain glossary.
- `docs/adr/`: architecture decision records.

## Before exploring

Read `CONTEXT.md` and ADRs relevant to the area being explored.

If these files do not exist, proceed silently without suggesting their
creation upfront. `/domain-modeling`, reached through `/grill-with-docs`
and `/improve-codebase-architecture`, creates them lazily when terms or
decisions are resolved.

## Use the glossary's vocabulary

Use terms defined in `CONTEXT.md` when naming domain concepts in issue
titles, proposals, hypotheses, and tests. Avoid synonyms the glossary excludes.

If a concept is missing, reconsider whether it belongs to the domain;
note genuine gaps for `/domain-modeling`.

## Flag ADR conflicts

Explicitly surface proposals that contradict an existing ADR, identifying
the ADR and explaining why the decision is worth reopening.
