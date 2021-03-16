#!/bin/sh

echo "hello"

#*****************************************************************
#************************ Building binary ******************
#*****************************************************************

go version
# echo $GOCACHE
# export GOCACHE=cache
env GOOS=linux go build -ldflags="-s -w" -o bin/covid-vaccine-warren main.go

zip covid-vaccine-warren.zip bin/covid-vaccine-warren

aws lambda update-function-code \
    --function-name  Covid-vaccine-warren-only \
    --zip-file fileb://./covid-vaccine-warren.zip