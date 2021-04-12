#!/bin/bash

cd /go/src/github.com/inburst/prty

go install ./... 
/go/bin/prty

inotifywait -e close_write,moved_to,create -m . |
while read -r directory events filename; do
  echo $filename
  #if [ "$filename" = "myfile.py" ]; then
  #  ./myfile.py
  #fi
done

