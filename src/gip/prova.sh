#!/usr/bin/env bash

for p in $(cat ~/.gip); do
    #echo "- $p"
    last=$(echo "$p" | awk -F/ '{ print $NF }')
    echo "  {
    \"Name\": \"$last\",
    \"Repository\": \"https://github.com/enr/$last.git\",
    \"Repository\": \"https://bitbucket.org/enr/$last.git\",
    \"Repository\": \"https://bitbucket.org/unicredit/$last.git\",
    \"Repository\": \"https://bitbucket.org/atoito/$last.git\",
    \"LocalPath\": \"~$p\"
  },"
done
