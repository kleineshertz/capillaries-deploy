# Sample .rc file to run before capildeploy 

Environment variables used in the .jsonnet file
```
# SSH access to EC2 instances
export CAPIDEPLOY_SSH_USER=ubuntu
# Keypair stored at AWS
export CAPIDEPLOY_AWS_SSH_ROOT_KEYPAIR_NAME=sampledeployment005-root-key
# Exported PEM file with private SSH key from the AWS keypair
export CAPIDEPLOY_SSH_PRIVATE_KEY_PATH=/home/johndoe/.ssh/sampledeployment005_rsa

# NGINX IP address filter
export CAPIDEPLOY_BASTION_ALLOWED_IPS=".../16"
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
export CAPIDEPLOY_S3_IAM_USER_AWS_ACCESS_KEY_ID=AK...
export CAPIDEPLOY_S3_IAM_USER_AWS_SECRET_ACCESS_KEY=...
# ~/.aws/config: default/region (without it, AWS API will not locate S3 buckets)
export CAPIDEPLOY_S3_AWS_DEFAULT_REGION=us-east-1

```
Role:
ARN: arn:aws:iam::728560144492:role/ec2-to-access-s3-capillaries-testbucket
Instance profile ARN: arn:aws:iam::728560144492:instance-profile/ec2-to-access-s3-capillaries-testbucket

Inline policy: s3-access-to-capillaries-testbucket
{
	"Version": "2012-10-17",
	"Statement": [
		{
			"Sid": "ListCapillariesTestbucket",
			"Effect": "Allow",
			"Action": "s3:ListBucket",
			"Resource": "arn:aws:s3:::capillaries-testbucket"
		},
		{
			"Sid": "GetPutDeleteCapillariesTestbucket",
			"Effect": "Allow",
            "Action": [
                "s3:DeleteObject",
                "s3:GetObject",
                "s3:PutObject"
            ],
            "Resource": "arn:aws:s3:::capillaries-testbucket/*"
		}
	]
}

$ aws iam get-instance-profile --instance-profile-name ec2-to-access-s3-capillaries-testbucket
{
    "InstanceProfile": {
        "Path": "/",
        "InstanceProfileName": "ec2-to-access-s3-capillaries-testbucket",
        "InstanceProfileId": "AIPA2TIMRSBWNMQA6JPDD",
        "Arn": "arn:aws:iam::728560144492:instance-profile/ec2-to-access-s3-capillaries-testbucket",
        "CreateDate": "2024-06-25T01:01:54+00:00",
        "Roles": [
            {
                "Path": "/",
                "RoleName": "ec2-to-access-s3-capillaries-testbucket",
                "RoleId": "AROA2TIMRSBWE76BODIBR",
                "Arn": "arn:aws:iam::728560144492:role/ec2-to-access-s3-capillaries-testbucket",
                "CreateDate": "2024-06-25T01:01:54+00:00",
                "AssumeRolePolicyDocument": {
                    "Version": "2012-10-17",
                    "Statement": [
                        {
                            "Effect": "Allow",
                            "Principal": {
                                "Service": "ec2.amazonaws.com"
                            },
                            "Action": "sts:AssumeRole"
                        }
                    ]
                }
            }
        ],
        "Tags": []
    }
}

$ aws iam list-instance-profiles-for-role --role-name ec2-to-access-s3-capillaries-testbucket
{
    "InstanceProfiles": [
        {
            "Path": "/",
            "InstanceProfileName": "ec2-to-access-s3-capillaries-testbucket",
            "InstanceProfileId": "AIPA2TIMRSBWNMQA6JPDD",
            "Arn": "arn:aws:iam::728560144492:instance-profile/ec2-to-access-s3-capillaries-testbucket",
            "CreateDate": "2024-06-25T01:01:54+00:00",
            "Roles": [
                {
                    "Path": "/",
                    "RoleName": "ec2-to-access-s3-capillaries-testbucket",
                    "RoleId": "AROA2TIMRSBWE76BODIBR",
                    "Arn": "arn:aws:iam::728560144492:role/ec2-to-access-s3-capillaries-testbucket",
                    "CreateDate": "2024-06-25T01:01:54+00:00",
                    "AssumeRolePolicyDocument": {
                        "Version": "2012-10-17",
                        "Statement": [
                            {
                                "Effect": "Allow",
                                "Principal": {
                                    "Service": "ec2.amazonaws.com"
                                },
                                "Action": "sts:AssumeRole"
                            }
                        ]
                    }
                }
            ]
        }
    ]
}

$ aws iam list-role-policies --role-name ec2-to-access-s3-capillaries-testbucket
{
    "PolicyNames": [
        "s3-access-to-capillaries-testbucket"
    ]
}

aws ec2 associate-iam-instance-profile --instance-id ... --iam-instance-profile Name=ec2-to-access-s3-capillaries-testbucket

CAPILLARIES_AWS_TESTBUCKET=capillaries-testbucket
keyspace="lookup_quicktest_s3"
cfgS3=s3://$CAPILLARIES_AWS_TESTBUCKET/capi_cfg/lookup_quicktest
outS3=s3://$CAPILLARIES_AWS_TESTBUCKET/capi_out/lookup_quicktest
scriptFile=$cfgS3/script.json
paramsFile=$cfgS3/script_params_one_run_s3.json
webapiUrl=http://$BASTION_IP:6544
startNodes=read_orders,read_order_items
curl -s -w "\n" -d '{"script_uri":"'$scriptFile'", "script_params_uri":"'$paramsFile'", "start_nodes":"'$startNodes'"}' -H "Content-Type: application/json" -X POST $webapiUrl"/ks/$keyspace/run"

