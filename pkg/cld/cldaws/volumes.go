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

const InitVolumeAttachmentFunc string = `
init_volume_attachment()
{ 
  local deviceName=$1
  local volumeMountPath=$2
  local permissions=$3
  local owner=$4

  # Check if file system is already there
  local deviceBlockId=$(blkid -s UUID -o value $deviceName)
  if [ "$deviceBlockId" = "" ]; then
    # Make file system (it outputs to stderr, so ignore it)
    sudo mkfs.ext4 $deviceName 2>/dev/null
    if [ "$?" -ne "0" ]; then
      echo Error $?, cannot make file system
      return $?
    fi
  fi

  deviceBlockId=$(sudo blkid -s UUID -o value $deviceName)

  local alreadyMounted=$(cat /etc/fstab | grep $volumeMountPath)

  if [ "$alreadyMounted" = "" ]; then
    # Create mount point
    sudo mkdir -p $volumeMountPath
    if [ "$?" -ne "0" ]; then
      echo Error $?, cannot create mount dir $volumeMountPath
      return $?
    fi

    # Set permissions
    sudo chmod $permissions $volumeMountPath
    if [ "$?" -ne "0" ]; then
		echo Error $?, cannot change $volumeMountPath permissions to $permissions
        return $?
    fi

	if [ -n "$owner" ]; then
	    sudo chown $owner $volumeMountPath
		if [ "$?" -ne "0" ]; then
			echo Error $?, cannot change $volumeMountPath owner to $owner
		    return $?
		fi
	fi

    # Adds a line to /etc/fstab
    echo "UUID=$deviceBlockId   $volumeMountPath   ext4   defaults   0   2 " | sudo tee -a /etc/fstab
  fi

  # Report UUID
  echo $deviceBlockId
  return 0
}
`

func GetVolumeIdByName(client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, volName string) (string, error) {
	if volName == "" {
		return "", fmt.Errorf("empty parameter not allowed: volName (%s)", volName)
	}
	out, err := client.DescribeVolumes(goCtx, &ec2.DescribeVolumesInput{
		Filters: []types.Filter{{Name: aws.String("tag:Name"), Values: []string{volName}}}})
	lb.AddObject("DescribeVolumes", out)
	if err != nil {
		return "", fmt.Errorf("cannot describe volume %s: %s", volName, err.Error())
	}
	if len(out.Volumes) == 0 {
		return "", nil
	}
	return *out.Volumes[0].VolumeId, nil
}

func GetVolumeAttachedDeviceById(client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, volId string) (string, types.VolumeAttachmentState, error) {
	if volId == "" {
		return "", types.VolumeAttachmentStateDetached, fmt.Errorf("empty parameter not allowed: volId (%s)", volId)
	}
	out, err := client.DescribeVolumes(goCtx, &ec2.DescribeVolumesInput{VolumeIds: []string{volId}})
	lb.AddObject("DescribeVolumes", out)
	if err != nil {
		return "", types.VolumeAttachmentStateDetached, fmt.Errorf("cannot describe volume by id %s: %s", volId, err.Error())
	}
	if len(out.Volumes) == 0 {
		return "", types.VolumeAttachmentStateDetached, nil
	}
	if len(out.Volumes[0].Attachments) == 0 {
		return "", types.VolumeAttachmentStateDetached, nil
	}
	return *out.Volumes[0].Attachments[0].Device, out.Volumes[0].Attachments[0].State, nil
}

func stringToVolType(volTypeString string) (types.VolumeType, error) {
	for _, volType := range types.VolumeTypeGp2.Values() {
		if string(volType) == volTypeString {
			return volType, nil
		}
	}
	return types.VolumeTypeStandard, fmt.Errorf("unknown volume type %s", volTypeString)
}

func CreateVolume(client *ec2.Client, goCtx context.Context, tags map[string]string, lb *l.LogBuilder, volName string, availabilityZone string, size int32, volTypeString string) (string, error) {
	volType, err := stringToVolType(volTypeString)
	if err != nil {
		return "", err
	}
	if volName == "" || availabilityZone == "" || size == 0 {
		return "", fmt.Errorf("empty parameter not allowed: volName (%s), availabilityZone (%s), size (%d)", volName, availabilityZone, size)
	}
	out, err := client.CreateVolume(goCtx, &ec2.CreateVolumeInput{
		AvailabilityZone: aws.String(availabilityZone),
		Size:             aws.Int32(size),
		VolumeType:       volType,
		TagSpecifications: []types.TagSpecification{{
			ResourceType: types.ResourceTypeVolume,
			Tags:         mapToTags(volName, tags)}}})
	lb.AddObject("CreateVolume", out)
	if err != nil {
		return "", fmt.Errorf("cannot create volume %s: %s", volName, err.Error())
	}
	return *out.VolumeId, nil
}

func AttachVolume(client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, volId string, instanceId string, suggestedDevice string, timeoutSeconds int) (string, error) {
	if volId == "" || instanceId == "" || suggestedDevice == "" {
		return "", fmt.Errorf("empty parameter not allowed: volId (%s), instanceId (%s), suggestedDevice (%s)", volId, instanceId, suggestedDevice)
	}
	out, err := client.AttachVolume(goCtx, &ec2.AttachVolumeInput{
		VolumeId:   aws.String(volId),
		InstanceId: aws.String(instanceId),
		Device:     &suggestedDevice})
	lb.AddObject("AttachVolume", out)
	if err != nil {
		return "", fmt.Errorf("cannot attach volume %s to instance %s as device %s : %s", volId, instanceId, suggestedDevice, err.Error())
	}

	newDevice := *out.Device

	startWaitTs := time.Now()
	for {
		foundDevice, state, err := GetVolumeAttachedDeviceById(client, goCtx, lb, volId)
		if err != nil {
			return "", err
		}
		if foundDevice != newDevice {
			return "", fmt.Errorf("cannot attach volume %s to instance %s as device %s : creation returned device %s, but while waiting discovered another device %s for this volume", volId, instanceId, suggestedDevice, newDevice, foundDevice)
		}
		if state == types.VolumeAttachmentStateAttached {
			break
		}
		if state != types.VolumeAttachmentStateAttaching {
			return "", fmt.Errorf("cannot attach volume %s to instance %s as device %s : unknown state %s", volId, instanceId, suggestedDevice, state)
		}
		if time.Since(startWaitTs).Seconds() > float64(timeoutSeconds) {
			return "", fmt.Errorf("giving up after waiting for volume %s to attach to instance %s as device %s", volId, instanceId, suggestedDevice)
		}
		time.Sleep(1 * time.Second)
	}

	return newDevice, nil
}

func DeleteVolume(client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, volId string) error {
	if volId == "" {
		return fmt.Errorf("empty parameter not allowed: volId (%s)", volId)
	}
	out, err := client.DeleteVolume(goCtx, &ec2.DeleteVolumeInput{VolumeId: aws.String(volId)})
	lb.AddObject("DeleteVolume", out)
	if err != nil {
		return fmt.Errorf("cannot delete volume %s: %s", volId, err.Error())
	}
	return nil
}
