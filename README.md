# IAM settings

## Capideploy operator user

### Policy
Create policy PolicyCapideployOperator:
```
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "PolicyCapideployOperatorCreateInfra",
            "Effect": "Allow",
            "Action": [
                "tag:GetResources",
                "iam:GetInstanceProfile",
                "ec2:DescribeAddresses",
                "ec2:AllocateAddress",
                "ec2:ReleaseAddress",
                "ec2:DescribeInstanceTypes",
                "ec2:DescribeImages",
                "ec2:DescribeImages",
                "ec2:DescribeKeyPairs",
                "ec2:DescribeInstances",
                "ec2:DescribeInstances",
                "ec2:RunInstances",
                "ec2:AssociateAddress",
                "ec2:TerminateInstances",
                "ec2:CreateImage",
                "ec2:DeregisterImage",
                "ec2:DeleteSnapshot",
                "ec2:AssociateIamInstanceProfile",
                "ec2:DescribeSubnets",
                "ec2:CreateSubnet",
                "ec2:DeleteSubnet",
                "ec2:DescribeVpcs",
                "ec2:CreateVpc",
                "ec2:DescribeVpcs",
                "ec2:DeleteVpc",
                "ec2:CreateRoute",
                "ec2:CreateRoute",
                "ec2:DescribeNatGateways",
                "ec2:CreateNatGateway",
                "ec2:DescribeNatGateways",
                "ec2:DeleteNatGateway",
                "ec2:DescribeNatGateways",
                "ec2:CreateRouteTable",
                "ec2:DescribeRouteTables",
                "ec2:DeleteRouteTable",
                "ec2:AssociateRouteTable",
                "ec2:DescribeInternetGateways",
                "ec2:CreateInternetGateway",
                "ec2:DeleteInternetGateway",
                "ec2:DescribeInternetGateways",
                "ec2:AttachInternetGateway",
                "ec2:DetachInternetGateway",
                "ec2:DescribeRouteTables",
                "ec2:DescribeAddresses",
                "ec2:DescribeVpcs",
                "ec2:DescribeSubnets",
                "ec2:DescribeSecurityGroups",
                "ec2:DescribeRouteTables",
                "ec2:DescribeInstances",
                "ec2:DescribeVolumes",
                "ec2:DescribeNatGateways",
                "ec2:DescribeInternetGateways",
                "ec2:DescribeImages",
                "ec2:DescribeSnapshots",
                "ec2:DescribeTags",
                "ec2:DescribeSecurityGroups",
                "ec2:CreateSecurityGroup",
                "ec2:AuthorizeSecurityGroupIngress",
                "ec2:DeleteSecurityGroup",
                "ec2:CreateTags",
                "ec2:DescribeVolumes",
                "ec2:DescribeVolumes",
                "ec2:CreateVolume",
                "ec2:AttachVolume",
                "ec2:DetachVolume",
                "ec2:DeleteVolume"
            ],
            "Resource": "*"
        },
        {
            "Sid": "PolicyCapideployOperatorPassAccessBucketRole",
            "Effect": "Allow",
            "Action": "iam:PassRole",
            "Resource": "arn:aws:iam::728560144492:role/RoleAccessCapillariesTestbucket"
        }
    ]
}
```

Without PassRole permission, AssociateIamInstanceProfile call will fail.


### User group with policy
Create group GroupCapideployOperators, go to Permissions tab, add PolicyCapideployOperator

### User in user group
Create user UserCapideployOperator, add it to GroupCapideployOperators. Create access key/secret for it, save to ~/UserCapideployOperator.rc:
```
export AWS_ACCESS_KEY_ID=AK...
export AWS_SECRET_ACCESS_KEY=...
export AWS_DEFAULT_REGION=us-east-1
```

## Access to cfg/in/out S3 bucket

### Policy

Create PolicyAccessCapillariesTestbucket
{
	"Version": "2012-10-17",
	"Statement": [
		{
			"Effect": "Allow",
			"Action": "s3:ListBucket",
			"Resource": "arn:aws:s3:::capillaries-testbucket"
		},
		{
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

### Role and instance profile

We do it so daemon/webapi/toolbelt running in the cloud can assume a role that allows S3 capillaries-testbucket access.

1. Create Role, select "AWS Sevice" -> EC2, name it RoleAccessCapillariesTestbucket.
2. In Permissions tab, attach PolicyAccessCapillariesTestbucket.
3. In Trust Relationship tab, make sure it says:
{
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
4. Make sure to export CAPIDEPLOY_INSTANCE_PROFILE_WITH_S3_ACCESS=RoleAccessCapillariesTestbucket, so capideploy tool picks it up.

Please note that RoleAccessCapillariesTestbucket has two ARN, as a role and as an instance profile
```
arn:aws:iam::728560144492:role/RoleAccessCapillariesTestbucket
arn:aws:iam::728560144492:instance-profile/RoleAccessCapillariesTestbucket
```
Run this command as AWS root or as UserCapideployOperator (who has iam:GetInstanceProfile permission, see above):
```
$ aws iam get-instance-profile --instance-profile-name RoleAccessCapillariesTestbucket
```
The result shows that role RoleAccessCapillariesTestbucket is "wrapped" by instance profile RoleAccessCapillariesTestbucket:
```json
{
    "InstanceProfile": {
        "Path": "/",
        "InstanceProfileName": "RoleAccessCapillariesTestbucket",
        "InstanceProfileId": "AIP...",
        "Arn": "arn:aws:iam::...:instance-profile/RoleAccessCapillariesTestbucket",
        "CreateDate": "...",
        "Roles": [
            {
                "Path": "/",
                "RoleName": "RoleAccessCapillariesTestbucket",
                "RoleId": "ARO...",
                "Arn": "arn:aws:iam::...:role/RoleAccessCapillariesTestbucket",
                "CreateDate": "...",
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
```

# Environment variables used by Capideploy

 Sample .rc file to run before Capildeploy contains variables used in the .jsonnet file:
```
# SSH access to EC2 instances
export CAPIDEPLOY_SSH_USER=ubuntu
# Keypair stored at AWS
export CAPIDEPLOY_AWS_SSH_ROOT_KEYPAIR_NAME=sampledeployment005-root-key
# Exported PEM file with private SSH key from the AWS keypair
export CAPIDEPLOY_SSH_PRIVATE_KEY_PATH=/home/johndoe/.ssh/sampledeployment005_rsa

# NGINX IP address filter
export CAPIDEPLOY_BASTION_ALLOWED_IPS=".../...,.../..."
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
# Capideploy writes it to ~/.aws/config: default/region
# Without it, AWS API will not locate S3 buckets
export CAPIDEPLOY_S3_AWS_DEFAULT_REGION=us-east-1

export CAPIDEPLOY_INSTANCE_PROFILE_WITH_S3_ACCESS=RoleAccessCapillariesTestbucket
```










random










Role:
ARN: arn:aws:iam::728560144492:role/RoleAccessCapillariesTestbucket 
Instance profile ARN: arn:aws:iam::728560144492:instance-profile/RoleAccessCapillariesTestbucket

It has attached policy PolicyAccessCapillariesTestbucket (arn:aws:iam::728560144492:policy/PolicyAccessCapillariesTestbucket):
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": "s3:ListBucket",
            "Resource": "arn:aws:s3:::capillaries-testbucket"
        },
        {
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

$ aws iam get-instance-profile --instance-profile-name RoleAccessCapillariesTestbucket
{
    "InstanceProfile": {
        "Path": "/",
        "InstanceProfileName": "RoleAccessCapillariesTestbucket",
        "InstanceProfileId": "AIPA2TIMRSBWNR5ODIPO7",
        "Arn": "arn:aws:iam::728560144492:instance-profile/RoleAccessCapillariesTestbucket",
        "CreateDate": "2024-06-26T02:13:15+00:00",
        "Roles": [
            {
                "Path": "/",
                "RoleName": "RoleAccessCapillariesTestbucket",
                "RoleId": "AROA2TIMRSBWIWAVXTLN2",
                "Arn": "arn:aws:iam::728560144492:role/RoleAccessCapillariesTestbucket",
                "CreateDate": "2024-06-26T02:13:15+00:00",
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
Similar output would be for "$ aws iam list-instance-profiles-for-role --role-name RoleAccessCapillariesTestbucket", but we do not give iam:ListInstanceProfilesForRole to the operator

Something like below would be for "$ aws iam list-role-policies --role-name RoleAccessCapillariesTestbucket", but we do not give iam:ListRolePolicies to the operator:
{
    "PolicyNames": [
        "RoleAccessCapillariesTestbucket"
    ]
}

aws ec2 associate-iam-instance-profile --instance-id ... --iam-instance-profile Name=RoleAccessCapillariesTestbucket

CAPILLARIES_AWS_TESTBUCKET=capillaries-testbucket
keyspace="lookup_quicktest_s3"
cfgS3=s3://$CAPILLARIES_AWS_TESTBUCKET/capi_cfg/lookup_quicktest
outS3=s3://$CAPILLARIES_AWS_TESTBUCKET/capi_out/lookup_quicktest
scriptFile=$cfgS3/script.json
paramsFile=$cfgS3/script_params_one_run_s3.json
webapiUrl=http://$BASTION_IP:6544
startNodes=read_orders,read_order_items
curl -s -w "\n" -d '{"script_uri":"'$scriptFile'", "script_params_uri":"'$paramsFile'", "start_nodes":"'$startNodes'"}' -H "Content-Type: application/json" -X POST $webapiUrl"/ks/$keyspace/run"

To give permission to associate an instance profile, you can define a policy like this (this example is using Terraform):


resource "aws_iam_policy" "example" {
  name        = "example"
  description = "Allow for assigning EC2 instance profile"
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow"
        Action   = "iam:GetInstanceProfile"
        Resource = <instance-profile-arn>
      },
      {
        Effect   = "Allow"
        Action   = "iam:PassRole"
        Resource = <role-used-for-instance-profile-arn>
      },
    ]
  })
}

grep -r -e "client\.[A-Za-z]*" --include "*.go"> ../a.txt





Create policy PolicyCapideployOperator:
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "PolicyCapideployOperator",
            "Effect": "Allow",
            "Action": [
                "tag:GetResources",
                "ec2:DescribeAddresses",
                "ec2:AllocateAddress",
                "ec2:ReleaseAddress",
                "ec2:DescribeInstanceTypes",
                "ec2:DescribeImages",
                "ec2:DescribeImages",
                "ec2:DescribeKeyPairs",
                "ec2:DescribeInstances",
                "ec2:DescribeInstances",
                "ec2:RunInstances",
                "ec2:AssociateAddress",
                "ec2:TerminateInstances",
                "ec2:CreateImage",
                "ec2:DeregisterImage",
                "ec2:DeleteSnapshot",
                "ec2:AssociateIamInstanceProfile",
                "ec2:DescribeSubnets",
                "ec2:CreateSubnet",
                "ec2:DeleteSubnet",
                "ec2:DescribeVpcs",
                "ec2:CreateVpc",
                "ec2:DescribeVpcs",
                "ec2:DeleteVpc",
                "ec2:CreateRoute",
                "ec2:CreateRoute",
                "ec2:DescribeNatGateways",
                "ec2:CreateNatGateway",
                "ec2:DescribeNatGateways",
                "ec2:DeleteNatGateway",
                "ec2:DescribeNatGateways",
                "ec2:CreateRouteTable",
                "ec2:DescribeRouteTables",
                "ec2:DeleteRouteTable",
                "ec2:AssociateRouteTable",
                "ec2:DescribeInternetGateways",
                "ec2:CreateInternetGateway",
                "ec2:DeleteInternetGateway",
                "ec2:DescribeInternetGateways",
                "ec2:AttachInternetGateway",
                "ec2:DetachInternetGateway",
                "ec2:DescribeRouteTables",
                "ec2:DescribeAddresses",
                "ec2:DescribeVpcs",
                "ec2:DescribeSubnets",
                "ec2:DescribeSecurityGroups",
                "ec2:DescribeRouteTables",
                "ec2:DescribeInstances",
                "ec2:DescribeVolumes",
                "ec2:DescribeNatGateways",
                "ec2:DescribeInternetGateways",
                "ec2:DescribeImages",
                "ec2:DescribeSnapshots",
                "ec2:DescribeTags",
                "ec2:DescribeSecurityGroups",
                "ec2:CreateSecurityGroup",
                "ec2:AuthorizeSecurityGroupIngress",
                "ec2:DeleteSecurityGroup",
                "ec2:CreateTags",
                "ec2:DescribeVolumes",
                "ec2:DescribeVolumes",
                "ec2:CreateVolume",
                "ec2:AttachVolume",
                "ec2:DetachVolume",
                "ec2:DeleteVolume"
            ],
            "Resource": "*"
        }
    ]
}

Create group GroupCapideployOperators, go to Permissions tab, add PolicyCapideployOperator
Create user UserCapideployOperator, add it to GroupCapideployOperators