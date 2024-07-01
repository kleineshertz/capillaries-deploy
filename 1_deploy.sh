##!/bin/bash

# Watch the progress:
# cat ./deploy.log | grep elapsed
# or
# less -R ./deploy.log

set -e # Exit on failure
set -x # Print commands

go build ./pkg/cmd/capideploy/capideploy.go

./capideploy list_deployment_resources -p sample.jsonnet -v > deploy.log

set +x
SECONDS=0
export BILLED_RESOURCES=$(cat deploy.log | grep ",billed")
if [ "$BILLED_RESOURCES" != "" ]; then
  echo "This deployment has resources that may be still/already active, please check the log"
fi

set -x # Print commands

./capideploy create_floating_ips -p sample.jsonnet -v >> deploy.log

set +x

# Save reserved BASTION_IP so we can run capitoolbelt on bastion
export BASTION_IP=$(cat deploy.log | grep "export BASTION_IP=" |  cut -d "=" -f2)
if [ "$BASTION_IP" = "" ]; then
  echo "Cannot retrieve BASTION_IP"
  exit 1
fi

# Configure SSH jumphost so we can run nodetool on Cassandra hosts (requires write access to ~/.ssh/config)
if ! grep -q "$BASTION_IP" ~/.ssh/config; then
  echo "Adding a new jumphost to ~/.ssh/config..."
  echo "" | tee -a ~/.ssh/config
  echo "Host $BASTION_IP" | tee -a ~/.ssh/config
  echo "  User $CAPIDEPLOY_SSH_USER" | tee -a ~/.ssh/config
  echo "  StrictHostKeyChecking=no" | tee -a ~/.ssh/config
  echo "  UserKnownHostsFile=/dev/null" | tee -a ~/.ssh/config
  echo "  IdentityFile  $CAPIDEPLOY_SSH_PRIVATE_KEY_PATH" | tee -a ~/.ssh/config
fi

set -x

./capideploy create_networking -p sample.jsonnet -v >> deploy.log
./capideploy create_security_groups -p sample.jsonnet -v >> deploy.log
./capideploy create_volumes "*" -p sample.jsonnet -v >> deploy.log
./capideploy create_instances "*" -p sample.jsonnet -v >> deploy.log
#./capideploy create_instances "bastion" -p sample.jsonnet -v >> deploy.log
./capideploy ping_instances '*' -p sample.jsonnet -n 20 >> deploy.log
#./capideploy ping_instances "bastion" -p sample.jsonnet -n 20 >> deploy.log
./capideploy attach_volumes "bastion" -p sample.jsonnet -v >> deploy.log

# install_services swaps sshd services, so do not use bastion as jumphost while it's in transition
./capideploy install_services "bastion" -p sample.jsonnet -v >> deploy.log
./capideploy install_services "rabbitmq,prometheus,daemon*,cass*" -p sample.jsonnet -v >> deploy.log

# Cassandra requires special treatment: stop and config/start
./capideploy stop_services "cass*" -p sample.jsonnet -v >> deploy.log
./capideploy config_services "cass*" -p sample.jsonnet -v >> deploy.log

./capideploy config_services "bastion,rabbitmq,prometheus,daemon*" -p sample.jsonnet -v >> deploy.log
#./capideploy config_services "bastion" -p sample.jsonnet -v >> deploy.log

exit 0

ssh -o StrictHostKeyChecking=no -i $CAPIDEPLOY_SSH_PRIVATE_KEY_PATH -J $BASTION_IP $CAPIDEPLOY_SSH_USER@10.5.0.11 'nodetool describecluster;nodetool status'

duration=$SECONDS
echo "$(($duration / 60))m $(($duration % 60))s elapsed."

set +x
echo To run commands against this deployment, you will probably need this:
echo export BASTION_IP=$BASTION_IP