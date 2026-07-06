FROM mcr.microsoft.com/devcontainers/base:trixie

RUN apt-get update \
  && apt-get install -y --no-install-recommends \
    just \
    vim \
  && rm -rf /var/lib/apt/lists/*
