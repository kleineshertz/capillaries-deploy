##!/bin/bash

set -e # Exit on failure
set -x # Print commands

./capideploy delete_snapshot_images "*" -p sample.jsonnet -v
./capideploy list_deployment_resources -p sample.jsonnet
