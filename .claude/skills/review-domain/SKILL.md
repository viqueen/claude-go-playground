---
description: Review a domain PR
argument-hint: <pr-number>
allowed-tools: Read, Bash, Glob, Grep
disable-model-invocation: true
context: fork
---

Read and follow the agent instructions from `.claude/agents/review-domain.md`.

## PR Context

- PR diff: !`gh pr diff $ARGUMENTS`
- PR info: !`gh pr view $ARGUMENTS`
