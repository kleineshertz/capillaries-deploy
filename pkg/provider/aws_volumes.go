package provider

import (
	"fmt"
	"sort"
	"strings"

	"github.com/capillariesio/capillaries-deploy/pkg/cld/cldaws"
	"github.com/capillariesio/capillaries-deploy/pkg/l"
	"github.com/capillariesio/capillaries-deploy/pkg/prj"
	"github.com/capillariesio/capillaries-deploy/pkg/rexec"
)

func (p *AwsDeployProvider) CreateVolume(iNickname string, volNickname string) (l.LogMsg, error) {
	lb := l.NewLogBuilder(l.CurFuncName(), p.GetCtx().IsVerbose)

	volDef := p.GetCtx().PrjPair.Live.Instances[iNickname].Volumes[volNickname]
	foundVolIdByName, err := cldaws.GetVolumeIdByName(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, volDef.Name)
	if err != nil {
		return lb.Complete(err)
	}

	if volDef.VolumeId == "" {
		// If it was already created, save it for future use, but do not create
		if foundVolIdByName != "" {
			lb.Add(fmt.Sprintf("volume %s(%s) already there, updating project", volDef.Name, foundVolIdByName))
			p.GetCtx().PrjPair.SetVolumeId(iNickname, volNickname, foundVolIdByName)
			return lb.Complete(nil)
		}
	} else {
		if foundVolIdByName == "" {
			// It was supposed to be there, but it's not present, complain
			return lb.Complete(fmt.Errorf("requested volume id %s not present, consider removing this id from the project file", volDef.VolumeId))
		} else if p.GetCtx().PrjPair.Live.Instances[iNickname].Volumes[volNickname].VolumeId != foundVolIdByName {
			// It is already there, but has different id, complain
			return lb.Complete(fmt.Errorf("requested volume id %s not matching existing volume id %s", volDef.VolumeId, foundVolIdByName))
		}
	}

	if volDef.VolumeId != "" {
		lb.Add(fmt.Sprintf("volume %s(%s) already there, no need to create", volDef.Name, foundVolIdByName))
		return lb.Complete(nil)
	}

	newId, err := cldaws.CreateVolume(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, p.GetCtx().Tags, lb, volDef.Name, volDef.AvailabilityZone, int32(volDef.Size), volDef.Type)
	if err != nil {
		return lb.Complete(err)
	}

	p.GetCtx().PrjPair.SetVolumeId(iNickname, volNickname, newId)

	return lb.Complete(nil)
}

// AWS hell https://stackoverflow.com/questions/70205661/correctly-specifying-device-name-for-ebs-volume-while-attaching-to-an-ec2-instan
func volNicknameToAwsSuggestedDeviceName(volumes map[string]*prj.VolumeDef, volNickname string) string {
	// Sorted list of vol nicknames
	volNicknames := make([]string, len(volumes))
	volCount := 0
	for volNickname := range volumes {
		volNicknames[volCount] = volNickname
		volCount++
	}
	sort.Slice(volNicknames, func(i, j int) bool { return volNicknames[i] > volNicknames[j] })
	volDeviceSuffix := 'f'
	for i := 0; i < len(volNicknames); i++ {
		if volNicknames[i] == volNickname {
			return "/dev/sd" + string(volDeviceSuffix)
		}
		volDeviceSuffix++
	}
	return "invalid-device-for-vol-" + volNickname
}

// Not used anymore, hopefully
// func awsFinalDeviceNameOld(suggestedDeviceName string) string {
// 	return strings.ReplaceAll(suggestedDeviceName, "/dev/sd", "/dev/xvd")
// }

func awsFinalDeviceNameNitro(suggestedDeviceName string) string {
	// See what lsblk shows for your case.
	// This is very hacky, but I didn't spend time to do it the right way
	deviceNameReplacer := strings.NewReplacer(
		"/dev/sdf", "/dev/nvme1n1",
		"/dev/sdg", "/dev/nvme2n1",
		"/dev/sdh", "/dev/nvme3n1")
	return deviceNameReplacer.Replace(suggestedDeviceName)
}

func (p *AwsDeployProvider) AttachVolume(iNickname string, volNickname string) (l.LogMsg, error) {
	lb := l.NewLogBuilder(l.CurFuncName(), p.GetCtx().IsVerbose)

	volDef := p.GetCtx().PrjPair.Live.Instances[iNickname].Volumes[volNickname]

	if volDef.MountPoint == "" || volDef.Permissions == 0 || volDef.Owner == "" {
		return lb.Complete(fmt.Errorf("empty parameter not allowed: volDef.MountPoint (%s), volDef.Permissions (%d), volDef.Owner (%s)", volDef.MountPoint, volDef.Permissions, volDef.Owner))
	}

	foundDevice, _, err := cldaws.GetVolumeAttachedDeviceById(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, volDef.VolumeId)
	if err != nil {
		return lb.Complete(err)
	}

	// Do not compare/complain, just overwrite: the number of attachment does not help catch unaccounted cloud resources anyways

	if volDef.Device != "" {
		if foundDevice != "" {
			lb.Add(fmt.Sprintf("volume %s already attached to %s, device %s, updating project", volNickname, iNickname, foundDevice))
		} else {
			lb.Add(fmt.Sprintf("volume %s was not attached to %s, cleaning attachment info, updating project", volNickname, iNickname))
		}
		p.GetCtx().PrjPair.SetAttachedVolumeDevice(iNickname, volNickname, foundDevice)
		return lb.Complete(nil)
	} else {
		if foundDevice != "" && foundDevice != volDef.Device {
			lb.Add(fmt.Sprintf("volume %s already to %s, but with a different device(%s->%s), updating project", volNickname, iNickname, volDef.Device, foundDevice))
			p.GetCtx().PrjPair.SetAttachedVolumeDevice(iNickname, volNickname, foundDevice)
			return lb.Complete(nil)
		}
	}

	suggestedDevice := volNicknameToAwsSuggestedDeviceName(p.GetCtx().PrjPair.Live.Instances[iNickname].Volumes, volNickname)

	// Attach

	instanceId := p.GetCtx().PrjPair.Live.Instances[iNickname].Id
	newDevice, err := cldaws.AttachVolume(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, volDef.VolumeId, instanceId, suggestedDevice, p.GetCtx().PrjPair.Live.Timeouts.AttachVolume)
	if err != nil {
		return lb.Complete(err)
	}

	p.GetCtx().PrjPair.SetAttachedVolumeDevice(iNickname, volNickname, newDevice)

	deviceBlockId, er := rexec.ExecSshAndReturnLastLine(
		p.GetCtx().PrjPair.Live.SshConfig,
		p.GetCtx().PrjPair.Live.Instances[iNickname].BestIpAddress(),
		fmt.Sprintf("%s\ninit_volume_attachment %s %s %d '%s'",
			cldaws.InitVolumeAttachmentFunc,
			awsFinalDeviceNameNitro(suggestedDevice), // AWS final device here
			volDef.MountPoint,
			volDef.Permissions,
			volDef.Owner))
	lb.Add(er.ToString())
	if er.Error != nil {
		return lb.Complete(fmt.Errorf("cannot mount volume %s to instance %s: %s", volNickname, iNickname, er.Error.Error()))
	}

	if deviceBlockId == "" || strings.HasPrefix(deviceBlockId, "Error") {
		return lb.Complete(fmt.Errorf("cannot mount volume %s to instance %s, returned blockDeviceId is: %s", volNickname, iNickname, deviceBlockId))
	}

	p.GetCtx().PrjPair.SetVolumeBlockDeviceId(iNickname, volNickname, deviceBlockId)

	return lb.Complete(nil)
}

func (p *AwsDeployProvider) DetachVolume(iNickname string, volNickname string) (l.LogMsg, error) {
	lb := l.NewLogBuilder(l.CurFuncName(), p.GetCtx().IsVerbose)

	volDef := p.GetCtx().PrjPair.Live.Instances[iNickname].Volumes[volNickname]
	if volDef.VolumeId == "" || volDef.Device == "" || volDef.MountPoint == "" {
		return lb.Complete(fmt.Errorf("empty parameter not allowed: volDef.VolumeId (%s), volDef.Device (%s), volDef.MountPoint (%s)", volDef.VolumeId, volDef.Device, volDef.MountPoint))
	}

	er := rexec.ExecSsh(
		p.GetCtx().PrjPair.Live.SshConfig,
		p.GetCtx().PrjPair.Live.Instances[iNickname].BestIpAddress(),
		fmt.Sprintf("sudo umount -d %s", volDef.MountPoint), map[string]string{})
	lb.Add(er.ToString())
	if er.Error != nil {
		return lb.Complete(fmt.Errorf("cannot umount volume %s on instance %s: %s", volNickname, iNickname, er.Error.Error()))
	}

	// er = rexec.ExecSsh(
	// 	p.GetCtx().PrjPair.Live.SshConfig,
	// 	p.GetCtx().PrjPair.Live.Instances[iNickname].BestIpAddress(),
	// 	fmt.Sprintf("sudo rm -fR %s", volDef.MountPoint), map[string]string{})
	// lb.Add(er.ToString())
	// if er.Error != nil {
	// 	return lb.Complete(fmt.Errorf("cannot delete mount point for volume %s on instance %s: %s", volNickname, iNickname, er.Error.Error()))
	// }

	instanceId := p.GetCtx().PrjPair.Live.Instances[iNickname].Id
	err := cldaws.DetachVolume(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, volDef.VolumeId, instanceId, volDef.Device)
	if err != nil {
		return lb.Complete(err)
	}

	p.GetCtx().PrjPair.SetVolumeBlockDeviceId(iNickname, volNickname, "")
	p.GetCtx().PrjPair.SetAttachedVolumeDevice(iNickname, volNickname, "")

	return lb.Complete(nil)
}

func (p *AwsDeployProvider) DeleteVolume(iNickname string, volNickname string) (l.LogMsg, error) {
	lb := l.NewLogBuilder(l.CurFuncName(), p.GetCtx().IsVerbose)

	volDef := p.GetCtx().PrjPair.Live.Instances[iNickname].Volumes[volNickname]
	foundVolIdByName, err := cldaws.GetVolumeIdByName(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, volDef.Name)
	if err != nil {
		return lb.Complete(err)
	}

	if foundVolIdByName == "" {
		lb.Add(fmt.Sprintf("volume %s not found, nothing to delete", volDef.Name))
		p.GetCtx().PrjPair.SetVolumeId(iNickname, volNickname, "")
		return lb.Complete(nil)
	}

	if foundVolIdByName != volDef.VolumeId {
		lb.Add(fmt.Sprintf("volume %s found, it has id %s, does not match known id %s", volDef.Name, foundVolIdByName, volDef.VolumeId))
		return lb.Complete(fmt.Errorf("cannot delete volume %s, it has id %s, does not match known id %s", volDef.Name, foundVolIdByName, volDef.VolumeId))
	}

	err = cldaws.DeleteVolume(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, volDef.VolumeId)
	if err != nil {
		return lb.Complete(err)
	}

	p.GetCtx().PrjPair.SetVolumeId(iNickname, volNickname, "")

	return lb.Complete(nil)
}
