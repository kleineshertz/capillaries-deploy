# Sample .rc file to run before capildeploy 

For up-to-date list, see "env_variables_used" in the .jsonnet file
```
# SSH access to EC2 instances
export CAPIDEPLOY_SSH_USER=ubuntu
# Keypair stored at AWS
export CAPIDEPLOY_AWS_SSH_ROOT_KEYPAIR_NAME=sampledeployment005-root-key
# Exported PEM file with private SSH key from the AWS keypair
export CAPIDEPLOY_SSH_PRIVATE_KEY_PATH=/home/johndoe/.ssh/sampledeployment005_rsa

# NGINX IP address filter
export CAPIDEPLOY_BASTION_ALLOWED_IPS="<my_subnet>/16"
export CAPIDEPLOY_EXTERNAL_WEBAPI_PORT=6544

# This is where capideploy takes Capillaries binaries from
export CAPIDEPLOY_CAPILLARIES_RELEASE_URL=https://capillaries-release.s3.us-east-1.amazonaws.com/latest

# RabbitMQ admin access (RabbitMQ Mgmt UI)
export CAPIDEPLOY_RABBITMQ_ADMIN_NAME=...
export CAPIDEPLOY_RABBITMQ_ADMIN_PASS=...

# RabbitMQ user access (used by Capillaries components to talk to RabbitMQ)
export CAPIDEPLOY_RABBITMQ_USER_NAME=...
export CAPIDEPLOY_RABBITMQ_USER_PASS=...

# arn:aws:iam::aws_account:user/capillaries-testuser to access s3
# ~/.aws/credentials: default/aws_access_key_id, default/aws_secret_access_key
export CAPIDEPLOY_IAM_AWS_ACCESS_KEY_ID=AK...
export CAPIDEPLOY_IAM_AWS_SECRET_ACCESS_KEY=...
# ~/.aws/config: default/region (without it, AWS API will not locate S3 buckets)
export CAPIDEPLOY_IAM_AWS_DEFAULT_REGION=us-east-1
```