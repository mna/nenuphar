#!/usr/bin/env bash

set -euo pipefail

# must run from the root of the repo
if [[ ! -s ./cmd/nenuphar/main.go ]]; then
  echo "make sure you are running from the root of the repository"
  exit 1
fi

# if there are any uncommitted changes, stop
if [[ -n "$(git status --porcelain)" ]]; then
  echo "git repository has uncommitted changes"
  exit 1
fi

# get the version information and current date
VERSION="$(git describe --tags 2> /dev/null || git rev-parse --short HEAD)"
DATE="$(date --iso-8601)"

mkdir -p ./bin

echo "${VERSION}"

#go build -o ./bin/nenuphar -ldflags "-X main.version=${VERSION} -X main.buildDate=${DATE}" ./cmd/nenuphar/
