##!/bin/bash

set +e # Continue on failure
set -x # Print commands

./capideploy stop_services "*" -p sample.jsonnet -v > undeploy.log

set -e # Exit on failure
./capideploy detach_volumes "bastion" -p sample.jsonnet -v >> undeploy.log
./capideploy delete_volumes "*" -p sample.jsonnet -v >> undeploy.log
./capideploy delete_instances "*" -p sample.jsonnet -v >> undeploy.log

./capideploy delete_security_groups -p sample.jsonnet -v >> undeploy.log
./capideploy delete_networking -p sample.jsonnet -v >> undeploy.log
./capideploy delete_floating_ips -p sample.jsonnet -v >> undeploy.log

./capideploy list_deployment_resources -p sample.jsonnet
