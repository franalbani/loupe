#!/bin/bash

counter=0

while true
do
  echo "stepout " $counter
  echo "steperr " $counter 1>&2
  sleep 0.1
  ((counter++))
done
