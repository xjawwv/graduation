# AGENTS.md

## Auto Commit

After completing changes in the repository, the agent MUST automatically create a commit.

### Rules

- Commit only files relevant to the current task.
- Never commit temporary files, caches, build outputs, credentials, `.env` files, secrets, or other sensitive data.
- Before committing, review the changes using `git status` and `git diff`.
- Ensure the requested changes are complete and do not leave any known errors.
- Run relevant tests, linting, type checks, or other validation when available.
- Do not run `git push` unless explicitly requested.
- Do not use `git commit --amend` or rewrite existing commit history unless explicitly requested.
- Never discard, reset, overwrite, or modify unrelated changes made by the user.
- If the repository contains unrelated changes, leave them untouched and exclude them from the commit.
- If there are no relevant changes to commit, do not create an empty commit.

### Commit Message

Use a short and descriptive commit message that clearly represents the changes.

Recommended format:

`<type>: <description>`

Available types:

- `feat`: new feature
- `fix`: bug fix
- `refactor`: code changes without behavior changes
- `docs`: documentation changes
- `test`: test changes
- `style`: formatting or style changes
- `chore`: maintenance or tooling changes
- `perf`: performance improvements

Examples:

`feat: add user authentication`

`fix: handle expired access tokens`

`refactor: simplify payment validation`

### Workflow

After completing a task:

1. Review `git status`.
2. Review `git diff`.
3. Run relevant validation.
4. Stage only files related to the current task.
5. Create a commit with an appropriate message.
6. Verify that the commit was created successfully.
7. Report the commit hash and commit message to the user.

Auto-commit is the default behavior. Do not ask for confirmation before committing unless the user explicitly requests that the changes remain uncommitted.