#!/usr/bin/env sh

# this is just a temporary script while debugging

GOOS=linux GOARCH=amd64 go build -tags lambda.norpc -o main ./...

rm ../../zip/pgDatabase.zip
zip ../../zip/pgDatabase.zip main

