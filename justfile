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
