#!/bin/sh

aws --region us-east-1 lambda invoke \
--function-name Covid-vaccine-warren-only \
text.json
