#!/bin/bash
set -e

PASSWD=$1
MATCHLOG=$2

go run main.go --match_log=$MATCHLOG

for F in `find leaderboard`; do
    if [ -f $F ]; then
        curl -F $F=@$F https://noleaversclub:$PASSWD@neocities.org/api/upload;
    fi
done
