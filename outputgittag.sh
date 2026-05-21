#!/bin/bash

# output current tags/commit to version
versions=()

# 讀取目前的標記 (tags) 到陣列
while IFS= read -r line || [ -n "$line" ]; do
    [ -n "$line" ] && versions+=("$line")
done < <(git tag --points-at HEAD)

# 讀取目前的提交雜湊 (commit hash) 到陣列
while IFS= read -r line || [ -n "$line" ]; do
    [ -n "$line" ] && versions+=("$line")
done < <(git log --pretty=format:'%h' -n 1)

echo -n "${versions[*]}" >version

# build with version
#go build -ldflags="-X 'github.com/bizshuk/gosdk/config.Version=$(cat version)'"
