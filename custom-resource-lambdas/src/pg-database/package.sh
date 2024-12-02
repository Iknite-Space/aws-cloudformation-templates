#!/usr/bin/env sh

# this is just a temporary script while debugging

GOOS=linux GOARCH=amd64 go build -tags lambda.norpc -o bootstrap pgDatabase.go  

zip ../../zip/pgDatabasse.zip bootstrap
