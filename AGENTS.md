# Repository Contribution Rules

## Update and Release Workflow

- Before changing code, fetch both configured upstream repositories and review all relevant updates and conflicts.
- Batch related fixes in the working tree. Do not commit or push after discovering each individual issue.
- Before the first push, run the complete applicable verification set: backend unit tests, integration tests, frontend checks, lint, security checks, and build validation.
- Fix all failures found during that verification pass, then rerun the affected checks and the full required checks until they are clean.
- Create one cohesive commit for the complete update and push it once. Do not use a fix-push-fix-push cycle for the same update.
- After pushing, wait for the complete remote CI and release workflows before reporting completion.
- Confirm the working tree is clean and the local branch is synchronized with its remote branch after any automated release-version commit.

## Upstream Merge Scope

- Keep the primary upstream and the selected `sub2api-custom` upstream changes clearly separated during review.
- Do not merge the shared account pool unless the user explicitly requests it.
- Record excluded upstream changes and unresolved conflicts before finalizing a merge.

## Commits and Releases

- Keep release-related fixes in the same pre-release commit when they belong to the same update.
- Never move or overwrite an existing release tag. Use the next valid date version when the current date tag is already occupied.
- Use SSH for repository pushes when the configured remote supports it.
