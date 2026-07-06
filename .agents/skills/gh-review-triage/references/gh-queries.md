# GH query patterns

Prefer `gh` CLI for PR review state. These patterns are enough for most review-triage tasks.

## Identify branch and PR

```bash
git branch --show-current
gh pr view --json number,title,headRefName,baseRefName,url
```

Use an explicit PR number when the user gives one:

```bash
gh pr view 123 --json number,title,headRefName,baseRefName,url
```

## Check authentication

```bash
gh auth status
```

## Fetch review threads with resolution state

Replace owner, repo, and PR number as needed:

```bash
gh api graphql -f query='query{repository(owner:"OWNER",name:"REPO"){pullRequest(number:PR_NUMBER){reviewThreads(first:100){nodes{isResolved isOutdated comments(first:20){nodes{author{login}body path line createdAt url}}}}}}}'
```

Use this to isolate unresolved comments in your own analysis after fetching the full thread set.

## Practical extraction notes

- `isResolved: false` means still open.
- `isOutdated: true` often means the file changed enough that the comment may no longer apply.
- The first comment body usually carries the main claim; later thread comments may add context or indicate disagreement.

## Minimal fields to preserve in a saved document

- PR number and URL
- thread resolution state: `isResolved`, `isOutdated`
- author
- severity/category when present
- file path and line
- timestamp
- full substantive body
- suggested diff or prompt block if present
- thread URL

## Recommended sequence

1. `gh auth status`
2. `git branch --show-current`
3. `gh pr view ...`
4. `gh api graphql ... reviewThreads ...`
5. inspect referenced files with local tools
