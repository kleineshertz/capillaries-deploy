package cldaws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/capillariesio/capillaries-deploy/pkg/l"
)

func StringToInstanceType(instanceTypeString string) (types.InstanceType, error) {
	for _, instanceType := range types.InstanceTypeT2Nano.Values() {
		if string(instanceType) == instanceTypeString {
			return instanceType, nil
		}
	}
	return types.InstanceTypeT2Nano, fmt.Errorf("unknown instance type %s", instanceTypeString)
}

func GetInstanceType(client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, flavorName string) (string, error) {
	out, err := client.DescribeInstanceTypes(goCtx, &ec2.DescribeInstanceTypesInput{Filters: []types.Filter{{
		Name: aws.String("instance-type"), Values: []string{flavorName}}}})
	lb.AddObject(out)
	if err != nil {
		return "", fmt.Errorf("cannot find flavor %s:%s", flavorName, err.Error())
	}
	if len(out.InstanceTypes) == 0 {
		return "", fmt.Errorf("found zero results for flavor %s", flavorName)
	}
	return string(out.InstanceTypes[0].InstanceType), nil // "t2.2xlarge"
}

func VerifyImageId(client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, imageId string) (string, error) {
	out, err := client.DescribeImages(goCtx, &ec2.DescribeImagesInput{Filters: []types.Filter{{
		Name: aws.String("image-id"), Values: []string{imageId}}}})
	lb.AddObject(out)
	if err != nil {
		return "", fmt.Errorf("cannot find image %s:%s", imageId, err.Error())
	}
	if len(out.Images) == 0 {
		return "", fmt.Errorf("found zero results for image %s", imageId)
	}
	return *out.Images[0].ImageId, nil
}

func VerifyKeypair(client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, keypairName string) error {
	out, err := client.DescribeKeyPairs(goCtx, &ec2.DescribeKeyPairsInput{Filters: []types.Filter{{
		Name: aws.String("key-name"), Values: []string{keypairName}}}})
	lb.AddObject(out)
	if err != nil {
		return fmt.Errorf("cannot find keypair %s:%s", keypairName, err.Error())
	}
	if len(out.KeyPairs) == 0 {
		return fmt.Errorf("found zero keypairs %s", keypairName)
	}
	return nil
}

func GetInstanceIdByHostName(client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, hostName string) (string, error) {
	out, err := client.DescribeInstances(goCtx, &ec2.DescribeInstancesInput{Filters: []types.Filter{types.Filter{
		Name: aws.String("tag:Name"), Values: []string{hostName}}}})
	lb.AddObject(out)
	if err != nil {
		return "", fmt.Errorf("cannot find instance %s:%s", hostName, err.Error())
	}
	if len(out.Reservations) == 0 {
		return "", nil
	}
	if len(out.Reservations[0].Instances) == 0 {
		return "", fmt.Errorf("found zero instances in reservations[0] for hostname %s", hostName)
	}
	return *out.Reservations[0].Instances[0].InstanceId, nil
}

func getInstanceStateName(client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, instanceId string) (types.InstanceStateName, error) {
	out, err := client.DescribeInstances(goCtx, &ec2.DescribeInstancesInput{InstanceIds: []string{instanceId}})
	lb.AddObject(out)
	if err != nil {
		return "", fmt.Errorf("cannot find instance %s:%s", instanceId, err.Error())
	}
	if len(out.Reservations) == 0 {
		return "", nil
	}
	if len(out.Reservations[0].Instances) == 0 {
		return "", fmt.Errorf("found zero instances in reservations[0] for hostname %s", instanceId)
	}
	return out.Reservations[0].Instances[0].State.Name, nil
}

func CreateInstance(client *ec2.Client, goCtx context.Context, lb *l.LogBuilder,
	instanceTypeString string,
	imageId string,
	hostName string,
	privateIpAddress string,
	securityGroupId string,
	rootKeyName string,
	subnetId string,
	timeoutSeconds int) (string, error) {

	instanceType, err := StringToInstanceType(instanceTypeString)
	if err != nil {
		return "", err
	}

	if imageId == "" || hostName == "" || privateIpAddress == "" || securityGroupId == "" || rootKeyName == "" || subnetId == "" {
		return "", fmt.Errorf("instance imageId(%s), hostname(%s), ip address(%s), security group id(%s), rook key name(%s), subnet id(%s) cannot be empty",
			imageId, hostName, privateIpAddress, securityGroupId, rootKeyName, subnetId)
	}

	// Start instance

	// NOTE: AWS doesn't allow to specify hostname on creation
	runOut, err := client.RunInstances(goCtx, &ec2.RunInstancesInput{
		InstanceType:     instanceType,
		ImageId:          aws.String(imageId),
		MinCount:         aws.Int32(1),
		MaxCount:         aws.Int32(1),
		KeyName:          aws.String(rootKeyName),
		SecurityGroupIds: []string{securityGroupId},
		SubnetId:         aws.String(subnetId),
		PrivateIpAddress: aws.String(privateIpAddress),
		TagSpecifications: []types.TagSpecification{{
			ResourceType: types.ResourceTypeInstance,
			Tags:         []types.Tag{{Key: aws.String("Name"), Value: aws.String(hostName)}},
		}}})
	lb.AddObject(runOut)
	if err != nil {
		return "", fmt.Errorf("cannot create instance %s: %s", hostName, err.Error())
	}
	if len(runOut.Instances) == 0 {
		return "", fmt.Errorf("got zero instances when creating %s", hostName)
	}

	newId := *runOut.Instances[0].InstanceId

	if newId == "" {
		return "", fmt.Errorf("aws returned empty instance id for %s", hostName)
	}

	startWaitTs := time.Now()
	for {
		stateName, err := getInstanceStateName(client, goCtx, lb, newId)
		if err != nil {
			return "", err
		}
		if stateName == types.InstanceStateNameRunning {
			break
		}
		if stateName != types.InstanceStateNamePending {
			return "", fmt.Errorf("%s(%s) was built, but the status is unknown: %s", hostName, newId, stateName)
		}
		if time.Since(startWaitTs).Seconds() > float64(timeoutSeconds) {
			return "", fmt.Errorf("giving up after waiting for %s(%s) to be created", hostName, newId)
		}
		time.Sleep(1 * time.Second)
	}
	return newId, nil
}

func AssignAwsFloatingIp(client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, instanceId string, floatingIp string) (string, error) {
	out, err := client.AssociateAddress(goCtx, &ec2.AssociateAddressInput{
		InstanceId: aws.String(instanceId),
		PublicIp:   aws.String(floatingIp)})
	lb.AddObject(out)
	if err != nil {
		return "", fmt.Errorf("cannot assign public IP %s to %s: %s", floatingIp, instanceId, err.Error())
	}
	if *out.AssociationId == "" {
		return "", fmt.Errorf("assigning public IP %s to %s returned empty association id", floatingIp, instanceId)
	}
	return *out.AssociationId, nil
}

func DeleteInstance(client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, instanceId string, timeoutSeconds int) error {
	out, err := client.TerminateInstances(goCtx, &ec2.TerminateInstancesInput{InstanceIds: []string{instanceId}})
	lb.AddObject(out)
	if err != nil {
		return fmt.Errorf("cannot delete instance %s: %s", instanceId, err.Error())
	}
	if len(out.TerminatingInstances) == 0 {
		return fmt.Errorf("got zero terminating instances when deleting %s", instanceId)
	}

	startWaitTs := time.Now()
	for {
		stateName, err := getInstanceStateName(client, goCtx, lb, instanceId)
		if err != nil {
			return err
		}
		if stateName == types.InstanceStateNameTerminated {
			break
		}
		if stateName != types.InstanceStateNameShuttingDown {
			return fmt.Errorf("%s was deleted, but the state is unknown: %s", instanceId, stateName)
		}
		if time.Since(startWaitTs).Seconds() > float64(timeoutSeconds) {
			return fmt.Errorf("giving up after waiting for %s to be deleted", instanceId)
		}
		time.Sleep(1 * time.Second)
	}
	return nil
}
