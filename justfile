set shell := ["pwsh.exe", "-NoProfile", "-c"]

# Default task prints help message
default: help

# Prints help message
help:
    @just --list

# Checks markdown files for linting and formatting errors
check-md:
    @pnpx markdownlint-cli2 .

# Fixes markdown files for linting and formatting errors
fix-md:
    @pnpx markdownlint-cli2 --fix .

# Checks Go packages
check-go:
    $root = (Resolve-Path .).Path; $env:GOPATH = "$root\.tmp-gopath"; $env:GOMODCACHE = "$root\.tmp-gomodcache"; $env:GOCACHE = "$root\.tmp-gocache"; go test ./...

# Formats Go files
fmt-go:
    $root = (Resolve-Path .).Path; $env:GOPATH = "$root\.tmp-gopath"; $env:GOMODCACHE = "$root\.tmp-gomodcache"; $env:GOCACHE = "$root\.tmp-gocache"; gofmt -w main.go cmd internal

# Runs all repository checks
check: check-md check-go
