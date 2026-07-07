# Code coverage Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add repository-level code coverage commands and a CI gate that fails below 80% total Go coverage.

**Architecture:** Reuse Go's built-in coverage tooling from `go test` and `go tool cover`. Keep logic in `justfile` so local development and GitHub Actions share one entrypoint. Ignore generated coverage artifacts in git.

**Tech Stack:** Go toolchain, just, PowerShell for percentage parsing in `just`, GitHub Actions

---

## Task 1: Add local coverage commands

**Files:**

- Modify: `justfile`

- [ ] **Step 1: Add recipe for generating coverage profile and text summary**

```just
coverage:
    @go test -coverprofile=coverage.out ./...
    @go tool cover -func=coverage.out
```

- [ ] **Step 2: Add recipe for generating HTML report**

```just
coverage-html:
    @go test -coverprofile=coverage.out ./...
    @go tool cover -html=coverage.out -o coverage.html
```

- [ ] **Step 3: Add recipe for enforcing coverage threshold**

```just
check-coverage:
    @go test -coverprofile=coverage.out ./...
    @pwsh -NoLogo -Command "$coverage = go tool cover -func=coverage.out | Select-String 'total:' | ForEach-Object { [double](($_.Matches[0].Groups[1].Value)) }; Write-Host ('Total coverage: {0:N1}%%' -f $coverage); if ($coverage -lt 80) { Write-Error ('Coverage {0:N1}% below threshold 80%' -f $coverage); exit 1 }""
```

## Task 2: Ignore generated artifacts and wire CI

**Files:**

- Modify: `.gitignore`
- Modify: `.github/workflows/ci.yml`

- [ ] **Step 1: Ignore generated coverage artifacts**

```gitignore
coverage.out
coverage.html
```

- [ ] **Step 2: Make GitHub Actions call coverage gate**

```yaml
      - name: Run Go checks
        run: just check-coverage
```

## Task 3: Verify behavior

**Files:**

- Test: local commands only

- [ ] **Step 1: Run coverage gate**

```bash
just check-coverage
```

Expected: exit code `0`, coverage summary printed, no threshold failure.

- [ ] **Step 2: Run HTML report recipe**

```bash
just coverage-html
```

Expected: `coverage.out` and `coverage.html` created locally.
