package cldaws

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/capillariesio/capillaries-deploy/pkg/l"
)

func stringToInstanceType(instanceTypeString string) (types.InstanceType, error) {
	for _, instanceType := range types.InstanceTypeT2Nano.Values() {
		if string(instanceType) == instanceTypeString {
			return instanceType, nil
		}
	}
	return types.InstanceTypeT2Nano, fmt.Errorf("unknown instance type %s", instanceTypeString)
}

func GetInstanceType(ec2Client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, flavorName string) (string, error) {
	out, err := ec2Client.DescribeInstanceTypes(goCtx, &ec2.DescribeInstanceTypesInput{
		InstanceTypes: []types.InstanceType{types.InstanceType(flavorName)}})
	lb.AddObject(fmt.Sprintf("DescribeInstanceTypes(InstanceType=%s)", flavorName), out)
	if err != nil {
		return "", fmt.Errorf("cannot find flavor %s:%s", flavorName, err.Error())
	}
	if len(out.InstanceTypes) == 0 {
		return "", fmt.Errorf("found zero results for flavor %s", flavorName)
	}
	return string(out.InstanceTypes[0].InstanceType), nil // "t2.2xlarge"
}

func GetImageInfoById(ec2Client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, imageId string) (types.ImageState, []types.BlockDeviceMapping, error) {
	out, err := ec2Client.DescribeImages(goCtx, &ec2.DescribeImagesInput{Filters: []types.Filter{{
		Name: aws.String("image-id"), Values: []string{imageId}}}})
	lb.AddObject(fmt.Sprintf("DescribeImages(image-id=%s)", imageId), out)
	if err != nil {
		return "", nil, fmt.Errorf("cannot find image %s:%s", imageId, err.Error())
	}
	if len(out.Images) == 0 {
		return "", nil, fmt.Errorf("found zero results for image %s", imageId)
	}
	return out.Images[0].State, out.Images[0].BlockDeviceMappings, nil
}

func GetImageInfoByName(ec2Client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, imageName string) (string, types.ImageState, []types.BlockDeviceMapping, error) {
	out, err := ec2Client.DescribeImages(goCtx, &ec2.DescribeImagesInput{Filters: []types.Filter{{
		Name: aws.String("tag:Name"), Values: []string{imageName}}}})
	lb.AddObject(fmt.Sprintf("DescribeImages(tag:Name=%s)", imageName), out)
	if err != nil {
		return "", "", nil, fmt.Errorf("cannot find image %s:%s", imageName, err.Error())
	}
	if len(out.Images) == 0 {
		return "", "", nil, nil
	}
	return *out.Images[0].ImageId, out.Images[0].State, out.Images[0].BlockDeviceMappings, nil
}

func VerifyKeypair(ec2Client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, keypairName string) error {
	out, err := ec2Client.DescribeKeyPairs(goCtx, &ec2.DescribeKeyPairsInput{Filters: []types.Filter{{
		Name: aws.String("key-name"), Values: []string{keypairName}}}})
	lb.AddObject(fmt.Sprintf("DescribeKeyPairs(key-name=%s)", keypairName), out)
	if err != nil {
		return fmt.Errorf("cannot find keypair %s:%s", keypairName, err.Error())
	}
	if len(out.KeyPairs) == 0 {
		return fmt.Errorf("found zero keypairs %s", keypairName)
	}
	return nil
}

func GetInstanceIdAndStateByHostName(ec2Client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, instName string) (string, types.InstanceStateName, error) {
	out, err := ec2Client.DescribeInstances(goCtx, &ec2.DescribeInstancesInput{Filters: []types.Filter{{Name: aws.String("tag:Name"), Values: []string{instName}}}})
	lb.AddObject(fmt.Sprintf("DescribeInstances(tag:Name=%s)", instName), out)
	if err != nil {
		return "", types.InstanceStateNameTerminated, fmt.Errorf("cannot find instance by name %s:%s", instName, err.Error())
	}
	if len(out.Reservations) == 0 {
		return "", types.InstanceStateNameTerminated, nil
	}
	if len(out.Reservations[0].Instances) == 0 {
		return "", types.InstanceStateNameTerminated, fmt.Errorf("found zero instances in reservations[0] for hostinstNamename %s", instName)
	}

	// If there are more than one instance, we want to return the one which is Running, or at least Pending
	var instanceId string
	var instanceStateName string
	for resIdx := 0; resIdx < len(out.Reservations); resIdx++ {
		for instIdx := 0; instIdx < len(out.Reservations[resIdx].Instances); instIdx++ {
			inst := out.Reservations[resIdx].Instances[instIdx]
			if inst.State.Name == types.InstanceStateNameRunning {
				return *inst.InstanceId, inst.State.Name, nil
			}
			if inst.State.Name == types.InstanceStateNamePending {
				instanceId = *inst.InstanceId
				instanceStateName = string(inst.State.Name)
			} else if instanceStateName != string(types.InstanceStateNamePending) {
				instanceId = *inst.InstanceId
				instanceStateName = string(inst.State.Name)
			}
		}
	}
	return instanceId, types.InstanceStateName(instanceStateName), nil
}

func getInstanceStateName(ec2Client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, instanceId string) (types.InstanceStateName, error) {
	out, err := ec2Client.DescribeInstances(goCtx, &ec2.DescribeInstancesInput{InstanceIds: []string{instanceId}})
	lb.AddObject(fmt.Sprintf("DescribeInstances(instanceId=%s)", instanceId), out)
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			return "", nil
		}
		return "", fmt.Errorf("cannot find instance by id %s:%s", instanceId, err.Error())
	}
	if len(out.Reservations) == 0 {
		return "", nil
	}
	if len(out.Reservations[0].Instances) == 0 {
		return "", fmt.Errorf("found zero instances in reservations[0] for instanceId %s", instanceId)
	}

	for resIdx := 0; resIdx < len(out.Reservations); resIdx++ {
		for instIdx := 0; instIdx < len(out.Reservations[resIdx].Instances); instIdx++ {
			inst := out.Reservations[resIdx].Instances[instIdx]
			if *inst.InstanceId == instanceId {
				return out.Reservations[resIdx].Instances[instIdx].State.Name, nil
			}
		}
	}
	return "", nil
}

func CreateInstance(ec2Client *ec2.Client, goCtx context.Context, tags map[string]string, lb *l.LogBuilder,
	instanceTypeString string,
	imageId string,
	instName string,
	privateIpAddress string,
	securityGroupId string,
	rootKeyName string,
	subnetId string,
	blockDeviceMappings []types.BlockDeviceMapping,
	timeoutSeconds int) (string, error) {

	instanceType, err := stringToInstanceType(instanceTypeString)
	if err != nil {
		return "", err
	}

	if imageId == "" || instName == "" || privateIpAddress == "" || securityGroupId == "" || rootKeyName == "" || subnetId == "" {
		return "", fmt.Errorf("empty parameter not allowed: imageId (%s), instName (%s), privateIpAddress (%s), securityGroupId (%s), rootKeyName (%s), subnetId (%s)",
			imageId, instName, privateIpAddress, securityGroupId, rootKeyName, subnetId)
	}

	// NOTE: AWS doesn't allow to specify hostname on creation, it assigns names like "ip-10-5-0-11"
	runOut, err := ec2Client.RunInstances(goCtx, &ec2.RunInstancesInput{
		InstanceType:        instanceType,
		ImageId:             aws.String(imageId),
		MinCount:            aws.Int32(1),
		MaxCount:            aws.Int32(1),
		KeyName:             aws.String(rootKeyName),
		SecurityGroupIds:    []string{securityGroupId},
		SubnetId:            aws.String(subnetId),
		PrivateIpAddress:    aws.String(privateIpAddress),
		BlockDeviceMappings: blockDeviceMappings,
		TagSpecifications: []types.TagSpecification{{
			ResourceType: types.ResourceTypeInstance,
			Tags:         mapToTags(instName, tags)}}})
	lb.AddObject(fmt.Sprintf("RunInstances(InstanceType=%s,ImageId=%s,tag:Name=%s)", instanceType, imageId, instName), runOut)
	if err != nil {
		return "", fmt.Errorf("cannot create instance %s: %s", instName, err.Error())
	}
	if len(runOut.Instances) == 0 {
		return "", fmt.Errorf("got zero instances when creating %s", instName)
	}

	newId := *runOut.Instances[0].InstanceId

	if newId == "" {
		return "", fmt.Errorf("aws returned empty instance id for %s", instName)
	}

	startWaitTs := time.Now()
	for {
		stateName, err := getInstanceStateName(ec2Client, goCtx, lb, newId)
		if err != nil {
			return "", err
		}
		// If no state name returned - the instance creation has just began, give it some time
		if stateName != "" {
			if stateName == types.InstanceStateNameRunning {
				break
			}
			if stateName != types.InstanceStateNamePending {
				return "", fmt.Errorf("%s(%s) was built, but the status is unknown: %s", instName, newId, stateName)
			}
		}
		if time.Since(startWaitTs).Seconds() > float64(timeoutSeconds) {
			return "", fmt.Errorf("giving up after waiting for %s(%s) to be created", instName, newId)
		}
		time.Sleep(1 * time.Second)
	}
	return newId, nil
}

func AssignAwsFloatingIp(ec2Client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, instanceId string, ipAddress string) (string, error) {
	out, err := ec2Client.AssociateAddress(goCtx, &ec2.AssociateAddressInput{
		InstanceId: aws.String(instanceId),
		PublicIp:   aws.String(ipAddress)})
	lb.AddObject(fmt.Sprintf("AssociateAddress(instanceId=%s,ipAddress=%s)", instanceId, ipAddress), out)
	if err != nil {
		return "", fmt.Errorf("cannot assign public IP %s to %s: %s", ipAddress, instanceId, err.Error())
	}
	if *out.AssociationId == "" {
		return "", fmt.Errorf("assigning public IP %s to %s returned empty association id", ipAddress, instanceId)
	}
	return *out.AssociationId, nil
}

func DeleteInstance(ec2Client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, instanceId string, timeoutSeconds int) error {
	out, err := ec2Client.TerminateInstances(goCtx, &ec2.TerminateInstancesInput{InstanceIds: []string{instanceId}})
	lb.AddObject(fmt.Sprintf("TerminateInstances(instanceId=%s)", instanceId), out)
	if err != nil {
		return fmt.Errorf("cannot delete instance %s: %s", instanceId, err.Error())
	}
	if len(out.TerminatingInstances) == 0 {
		return fmt.Errorf("got zero terminating instances when deleting %s", instanceId)
	}

	startWaitTs := time.Now()
	for {
		stateName, err := getInstanceStateName(ec2Client, goCtx, lb, instanceId)
		if err != nil {
			return err
		}

		// If no state name returned - the instance is gone already (a bit too fast, but possible in theory)
		if stateName == "" {
			break
		}
		if stateName == types.InstanceStateNameTerminated {
			break
		}
		if stateName != types.InstanceStateNameShuttingDown && stateName != types.InstanceStateNameRunning {
			return fmt.Errorf("%s was deleted, but the state is unknown: %s", instanceId, stateName)
		}
		if time.Since(startWaitTs).Seconds() > float64(timeoutSeconds) {
			return fmt.Errorf("giving up after waiting for %s to be deleted", instanceId)
		}
		time.Sleep(1 * time.Second)
	}
	return nil
}

func StopInstance(ec2Client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, instanceId string, timeoutSeconds int) error {
	out, err := ec2Client.StopInstances(goCtx, &ec2.StopInstancesInput{InstanceIds: []string{instanceId}})
	lb.AddObject(fmt.Sprintf("StopInstances(instanceId=%s)", instanceId), out)
	if err != nil {
		return fmt.Errorf("cannot dtop instance %s: %s", instanceId, err.Error())
	}

	startWaitTs := time.Now()
	for {
		stateName, err := getInstanceStateName(ec2Client, goCtx, lb, instanceId)
		if err != nil {
			return err
		}

		if stateName == types.InstanceStateNameStopped {
			break
		}
		if stateName != types.InstanceStateNameStopping {
			return fmt.Errorf("cannot stop instance %s, uknown state: %s", instanceId, stateName)
		}
		if time.Since(startWaitTs).Seconds() > float64(timeoutSeconds) {
			return fmt.Errorf("giving up after waiting for instance %s to be stop", instanceId)
		}
		time.Sleep(1 * time.Second)
	}
	return nil
}

// aws ec2 create-image --region "us-east-1" --instance-id i-03c10fd5566a08476 --name ami-i-03c10fd5566a08476 --no-reboot
func CreateImageFromInstance(ec2Client *ec2.Client, goCtx context.Context, tags map[string]string, lb *l.LogBuilder, imageName string, instanceId string, timeoutSeconds int) (string, error) {
	out, err := ec2Client.CreateImage(goCtx, &ec2.CreateImageInput{
		InstanceId: aws.String(instanceId),
		Name:       aws.String(imageName),
		TagSpecifications: []types.TagSpecification{{
			ResourceType: types.ResourceTypeImage,
			Tags:         mapToTags(imageName, tags)}}})
	lb.AddObject(fmt.Sprintf("CreateImage(imageName=%s,instanceId=%s)", imageName, instanceId), out)
	if err != nil {
		return "", fmt.Errorf("cannot create snapshot image %s from instance %s: %s", imageName, instanceId, err.Error())
	}

	imageId := *out.ImageId

	startWaitTs := time.Now()
	for {
		state, _, err := GetImageInfoById(ec2Client, goCtx, lb, imageId)
		if err != nil {
			return "", err
		}
		if state == types.ImageStateAvailable {
			break
		}
		if state != types.ImageStatePending {
			return "", fmt.Errorf("image %s(%s) was built, but the status is unknown: %s", imageName, imageId, state)
		}
		if time.Since(startWaitTs).Seconds() > float64(timeoutSeconds) {
			return "", fmt.Errorf("giving up after waiting for image %s(%s) to be created for %ds", imageName, imageId, timeoutSeconds)
		}
		time.Sleep(1 * time.Second)
	}
	return imageId, nil
}

func DeregisterImage(ec2Client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, imageId string) error {
	out, err := ec2Client.DeregisterImage(goCtx, &ec2.DeregisterImageInput{ImageId: aws.String(imageId)})
	lb.AddObject(fmt.Sprintf("DeregisterImage(imageId=%s)", imageId), out)
	if err != nil {
		return fmt.Errorf("cannot delete image %s:%s", imageId, err.Error())
	}
	return nil
}

func DeleteSnapshot(ec2Client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, volSnapshotId string) error {
	out, err := ec2Client.DeleteSnapshot(goCtx, &ec2.DeleteSnapshotInput{SnapshotId: aws.String(volSnapshotId)})
	lb.AddObject(fmt.Sprintf("DeleteSnapshot(volSnapshotId=%s)", volSnapshotId), out)
	if err != nil {
		return fmt.Errorf("cannot delete snapshot %s:%s", volSnapshotId, err.Error())
	}
	return nil
}

func AssociateInstanceProfile(ec2Client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, instanceId string, instanceProfileName string) error {
	iamInstanceProfileSpec := types.IamInstanceProfileSpecification{}
	if strings.HasPrefix(instanceProfileName, "arn:aws:iam") {
		iamInstanceProfileSpec.Arn = aws.String(instanceProfileName)
	} else {
		iamInstanceProfileSpec.Name = aws.String(instanceProfileName)
	}
	out, err := ec2Client.AssociateIamInstanceProfile(goCtx, &ec2.AssociateIamInstanceProfileInput{
		InstanceId:         aws.String(instanceId),
		IamInstanceProfile: &iamInstanceProfileSpec})
	lb.AddObject(fmt.Sprintf("AssociateInstanceProfile(instanceId=%s,instanceProfileName=%s)", instanceId, instanceProfileName), out)
	if err != nil {
		return fmt.Errorf("cannot associate instance profile %s with %s: %s", instanceProfileName, instanceId, err.Error())
	}
	if *out.IamInstanceProfileAssociation.InstanceId == "" {
		return fmt.Errorf("associating instance profile %s with %s returned empty instance id", instanceProfileName, instanceId)
	}
	return nil
}
