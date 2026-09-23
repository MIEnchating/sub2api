# Repository Contribution Rules

## Update and Release Workflow

- Before changing code, fetch both configured upstream repositories and review all relevant updates and conflicts.
- Batch related fixes in the working tree. Do not commit or push after discovering each individual issue.
- Before the first push, run the complete applicable verification set: backend unit tests, integration tests, frontend checks, lint, security checks, and build validation.
- Fix all failures found during that verification pass, then rerun the affected checks and the full required checks until they are clean.
- Create one cohesive commit for the complete update and push it once. Do not use a fix-push-fix-push cycle for the same update.
- After pushing, wait for the complete remote CI and release workflows before reporting completion.
- Confirm the working tree is clean and the local branch is synchronized with its remote branch after any automated release-version commit.
- Scheduled upstream jobs must finish a complete validation pass and collect all failures before invoking Codex for one concentrated repair pass. After each repair, rerun the complete validation set.
- Scheduled jobs must actively repair merge conflicts and malformed review output before notifying an operator. Validate the entire decision schema before accepting a resolved review; retry using the previous output and error logs. Notify failure only after bounded repair attempts are exhausted or a concrete product decision is required, and preserve the failed candidate for recovery.
- Before merging upstream, save and push pending local changes on the target branch. Refuse unfinished Git operations or diverged history rather than force-pushing or discarding work. Dry runs must not commit or push.
- Sync notifications must contain an HTML alternative and a readable text alternative, including when detailed report rendering fails. Keep post-push release failures distinct from failures that left the candidate unpushed.
- On final failure, email the exact failed checks, key errors, likely direct causes, repair-attempt count, and confirmation that no candidate was pushed. On successful merge and push, email a success report.

## Upstream Merge Scope

- Merge all changes from the primary `upstream/main` branch without feature-by-feature filtering, except retired batch image generation as specified below.
- Merge all changes from the second `overdraft/sub2api-custom` branch except the shared account pool and batch image generation features and their supporting implementation.
- Do not selectively omit other second-upstream changes merely because they are large or unrelated; the shared account pool and batch image generation are excluded by the current product decision.
- Record the excluded shared-account-pool and batch-image-generation paths and any unresolved conflicts before finalizing a merge.

## Commits and Releases

- Keep release-related fixes in the same pre-release commit when they belong to the same update.
- Never move or overwrite an existing release tag. Use the next valid date version when the current date tag is already occupied.
- Use SSH for repository pushes when the configured remote supports it.

## Retired Features

- Batch image generation must not be reintroduced from either upstream. Exclude its UI, navigation, API, worker/queue, billing, configuration, tests, docs, and generated ORM implementation. Ordinary image generation and asynchronous single-image tasks remain supported.
- Preserve already shipped SQL migrations and their checksum compatibility entries; do not drop historical tables, records, or frozen balances as part of feature removal.
- Follow `docs/UPSTREAM_EXCLUSIONS.md` and run `python3 .github/check-upstream-exclusions.py .` after each merge and repair.
