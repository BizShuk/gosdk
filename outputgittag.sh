#!/bin/bash

# output current tags/commit to version
versions=($(git tag --points-at HEAD))
versions+=($(git log --pretty=format:'%h' -n 1))
echo -n "${versions[*]}" > version


# build with version
#go build -ldflags="-X 'github.com/bizshuk/gin_default/config.Version=$(cat version)'"
