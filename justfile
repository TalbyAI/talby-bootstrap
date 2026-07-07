# Default task prints help message
default: help

# Prints help message
help:
    @just --list

# Checks markdown files for linting and formatting errors
check-md:
    @npx -y markdownlint-cli2 .

# Fixes markdown files for linting and formatting errors
fix-md:
    @npx -y markdownlint-cli2 --fix .

# Checks Go packages
check-go:
    @go test ./...

# Generates Go coverage profile and text summary
coverage:
    @go test -coverprofile=coverage.out ./...
    @go tool cover -func=coverage.out

# Generates Go coverage HTML report
coverage-html:
    @go test -coverprofile=coverage.out ./...
    @go tool cover -html=coverage.out -o coverage.html

# Fails if total Go coverage is below threshold
check-coverage:
    @go test -coverprofile=coverage.out ./...
    @go tool cover -func=coverage.out | awk '/^total:/ { total = $3; sub(/%/, "", total); print "Total coverage: " total "%"; if (total + 0 < 80) { print "Coverage " total "% below threshold 80%" > "/dev/stderr"; exit 1 } }'

# Serves an HTML file from its directory and prints direct URL
serve-html file:
    @node scripts/serve-html.mjs {{file}}

# Generates Go coverage HTML report and serves it locally
coverage-serve:
    @just coverage-html
    @just serve-html coverage.html

# Runs the CLI with optional arguments
run *args:
    @go run . {{args}}

# Formats Go files
fmt-go:
    @gofmt -w main.go cmd internal

# Runs all repository checks
check: check-md check-go

# Runs all repository fixes
fix: fix-md fmt-go
