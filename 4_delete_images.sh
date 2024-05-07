##!/bin/bash

set -e # Exit on failure
set -x # Print commands

./capideploy delete_snapshot_images "*" -prj=sample.json --verbose
./capideploy list_deployment_resources -prj=./sample.json
