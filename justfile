set shell := ["pwsh.exe", "-NoProfile", "-c"]

# Default task prints help message
default: help

# Prints help message
help:
    @just --help

# Checks markdown files for linting and formatting errors
check-md:
    @pnpx markdownlint-cli2 .

# Fixes markdown files for linting and formatting errors
fix-md:
    @pnpx markdownlint-cli2 --fix .
