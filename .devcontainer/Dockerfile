FROM mcr.microsoft.com/devcontainers/go:2-1.26-trixie

RUN apt-get update \
  && apt-get install -y --no-install-recommends just \
  && rm -rf /var/lib/apt/lists/*