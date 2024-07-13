package prj

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/capillariesio/capillaries-deploy/pkg/rexec"
	"github.com/google/go-jsonnet"
)

type InstancePurpose string

const (
	InstancePurposeBastion    InstancePurpose = "CAPIDEPLOY.INTERNAL.PURPOSE_BASTION"
	InstancePurposeCassandra  InstancePurpose = "CAPIDEPLOY.INTERNAL.PURPOSE_CASSANDRA"
	InstancePurposeDaemon     InstancePurpose = "CAPIDEPLOY.INTERNAL.PURPOSE_DAEMON"
	InstancePurposeRabbitmq   InstancePurpose = "CAPIDEPLOY.INTERNAL.PURPOSE_RABBITMQ"
	InstancePurposePrometheus InstancePurpose = "CAPIDEPLOY.INTERNAL.PURPOSE_PROMETHEUS"
)

type ExecTimeouts struct {
	CreateInstance   int `json:"create_instance"`
	DeleteInstance   int `json:"delete_instance"`
	CreateNatGateway int `json:"create_nat_gateway"`
	DeleteNatGateway int `json:"delete_nat_gateway"`
	CreateNetwork    int `json:"create_network"`
	AttachVolume     int `json:"attach_volume"`
	DetachVolume     int `json:"detach_volume"`
	CreateImage      int `json:"create_image"`
	StopInstance     int `json:"stop_instance"`
}

func (t *ExecTimeouts) InitDefaults() {
	if t.CreateInstance == 0 {
		t.CreateInstance = 120
	}
	if t.DeleteInstance == 0 {
		t.DeleteInstance = 600
	}
	if t.CreateNatGateway == 0 {
		t.CreateNatGateway = 180 // It really may take that long
	}
	if t.DeleteNatGateway == 0 {
		t.DeleteNatGateway = 180 // It may take AWS a while
	}
	if t.CreateNetwork == 0 {
		t.CreateNetwork = 120
	}
	if t.AttachVolume == 0 {
		t.AttachVolume = 30
	}
	if t.DetachVolume == 0 {
		t.DetachVolume = 30
	}
	if t.CreateImage == 0 {
		t.CreateImage = 600
	}
	if t.StopInstance == 0 {
		t.StopInstance = 300
	}
}

type SecurityGroupRuleDef struct {
	Desc string `json:"desc"` // human-readable
	//Id        string `json:"id"`        // guid
	Protocol  string `json:"protocol"`  // tcp
	Ethertype string `json:"ethertype"` // IPv4
	RemoteIp  string `json:"remote_ip"` // 0.0.0.0/0
	Port      int    `json:"port"`      // 22
	Direction string `json:"direction"` // ingress
}

type SecurityGroupDef struct {
	Name string `json:"name"`
	//Id    string                  `json:"id"`
	Rules []*SecurityGroupRuleDef `json:"rules"`
}

// func (sg *SecurityGroupDef) Clean() {
// 	sg.Id = ""
// 	for _, r := range sg.Rules {
// 		r.Id = ""
// 	}
// }

type PrivateSubnetDef struct {
	Name string `json:"name"`
	//Id               string `json:"id"`
	Cidr                  string `json:"cidr"`
	RouteTableToNatgwName string `json:"route_table_to_nat_gateway_name"` // AWS only
	AvailabilityZone      string `json:"availability_zone"`               // AWS only
	//RouteTableToNat  string `json:"route_table_to_nat"` // AWS only
}

// AWS-specific
type PublicSubnetDef struct {
	Name                     string `json:"name"`
	Cidr                     string `json:"cidr"`
	AvailabilityZone         string `json:"availability_zone"`
	NatGatewayName           string `json:"nat_gateway_name"`
	NatGatewayExternalIpName string `json:"nat_gateway_external_ip_address_name"`
	//Id                       string //`json:"id"`
	//NatGatewayId         string //`json:"nat_gateway_id"`
	//NatGatewayExternalIp string //`json:"nat_gateway_public_ip"`
}

type RouterDef struct {
	Name string `json:"name"`
	//Id   string `json:"id"`
}

type NetworkDef struct {
	Name string `json:"name"`
	//Id            string           `json:"id"`
	Cidr          string           `json:"cidr"`
	PrivateSubnet PrivateSubnetDef `json:"private_subnet"`
	PublicSubnet  PublicSubnetDef  `json:"public_subnet"`
	Router        RouterDef        `json:"router"`
}

type VolumeDef struct {
	Name             string `json:"name"`
	MountPoint       string `json:"mount_point"`
	Size             int    `json:"size"`
	Type             string `json:"type"`
	Permissions      int    `json:"permissions"`
	Owner            string `json:"owner"`
	AvailabilityZone string `json:"availability_zone"`
	//VolumeId         string `json:"id"`
	//Device           string `json:"device"`
	//BlockDeviceId    string `json:"block_device_id"`
}

type ServiceCommandsDef struct {
	Install []string `json:"install"`
	Config  []string `json:"config"`
	Start   []string `json:"start"`
	Stop    []string `json:"stop"`
}
type ServiceDef struct {
	Env map[string]string  `json:"env"`
	Cmd ServiceCommandsDef `json:"cmd"`
}

type UserDef struct {
	Name          string `json:"name"`
	PublicKeyPath string `json:"public_key_path"`
}
type PrivateKeyDef struct {
	Name           string `json:"name"`
	PrivateKeyPath string `json:"private_key_path"`
}
type InstanceDef struct {
	Purpose  string `json:"purpose"`
	InstName string `json:"inst_name"`
	//SecurityGroupNickname string                `json:"security_group"`
	SecurityGroupName         string                `json:"security_group_name"`
	RootKeyName               string                `json:"root_key_name"`
	IpAddress                 string                `json:"ip_address"`
	ExternalIpAddressName     string                `json:"external_ip_address_name,omitempty"` // Populated for bastion only
	ExternalIpAddress         string                `json:"external_ip_address"`                // Output only, populated for bastion only
	FlavorName                string                `json:"flavor"`
	ImageId                   string                `json:"image_id"`
	SubnetName                string                `json:"subnet_name"`
	Volumes                   map[string]*VolumeDef `json:"volumes,omitempty"`
	Service                   ServiceDef            `json:"service"`
	AssociatedInstanceProfile string                `json:"associated_instance_profile"`
	//SubnetType            string                `json:"subnet_type"`
	//Id                    string                `json:"id"`
	//SnapshotImageId       string                `json:"snapshot_image_id"`
	//UsesSshConfigExternalIpAddress bool                  `json:"uses_ssh_config_external_ip_address,omitempty"`
}

func (iDef *InstanceDef) BestIpAddress() string {
	if iDef.ExternalIpAddressName != "" {
		if iDef.ExternalIpAddress == "" {
			return "you-did-not-call-ensurebastionip"
		}
		return iDef.ExternalIpAddress
	}
	return iDef.IpAddress
}

// func (iDef *InstanceDef) Clean() {
// 	iDef.Id = ""
// 	for _, volAttachDef := range iDef.Volumes {
// 		volAttachDef.Device = ""
// 		volAttachDef.BlockDeviceId = ""
// 		// Do not clean volAttachDef.VolumeId, it should be handled by delete_volumes
// 	}
// }

type Project struct {
	DeploymentName     string                       `json:"deployment_name"`
	SshConfig          *rexec.SshConfigDef          `json:"ssh_config"`
	Timeouts           ExecTimeouts                 `json:"timeouts"`
	SecurityGroups     map[string]*SecurityGroupDef `json:"security_groups"`
	Network            NetworkDef                   `json:"network"`
	Instances          map[string]*InstanceDef      `json:"instances"`
	DeployProviderName string                       `json:"deploy_provider_name"`
	// EnvVariablesUsed   []string                     `json:"env_variables_used"`
}

func (p *Project) InitDefaults() {
	p.Timeouts.InitDefaults()
}

const DeployProviderAws string = "aws"

type ProjectPair struct {
	// Template Project
	Live Project
	// ProjectFileDirPath string
}

// func (prjPair *ProjectPair) SetSecurityGroupId(sgNickname string, newId string) {
// 	prjPair.Template.SecurityGroups[sgNickname].Id = newId
// 	prjPair.Live.SecurityGroups[sgNickname].Id = newId
// }

// func (prjPair *ProjectPair) SetSecurityGroupRuleId(sgNickname string, ruleIdx int, newId string) {
// 	prjPair.Template.SecurityGroups[sgNickname].Rules[ruleIdx].Id = newId
// 	prjPair.Live.SecurityGroups[sgNickname].Rules[ruleIdx].Id = newId
// }

// func (prjPair *ProjectPair) CleanSecurityGroup(sgNickname string) {
// 	prjPair.Template.SecurityGroups[sgNickname].Clean()
// 	prjPair.Live.SecurityGroups[sgNickname].Clean()
// }

// func (prjPair *ProjectPair) SetNetworkId(newId string) {
// 	prjPair.Template.Network.Id = newId
// 	prjPair.Live.Network.Id = newId
// }

// func (prjPair *ProjectPair) SetRouterId(newId string) {
// 	prjPair.Template.Network.Router.Id = newId
// 	prjPair.Live.Network.Router.Id = newId
// }

// func (prjPair *ProjectPair) SetNatGatewayId(newId string) {
// 	prjPair.Template.Network.PublicSubnet.NatGatewayId = newId
// 	prjPair.Live.Network.PublicSubnet.NatGatewayId = newId
// }

// func (prjPair *ProjectPair) SetRouteTableToNat(newId string) {
// 	prjPair.Template.Network.PrivateSubnet.RouteTableToNat = newId
// 	prjPair.Live.Network.PrivateSubnet.RouteTableToNat = newId
// }

// func (prjPair *ProjectPair) SetPublicSubnetNatGatewayExternalIp(newIp string) {
// 	prjPair.Template.Network.PublicSubnet.NatGatewayExternalIp = newIp
// 	prjPair.Live.Network.PublicSubnet.NatGatewayExternalIp = newIp
// }

// func (prjPair *ProjectPair) SetPrivateSubnetId(newId string) {
// 	prjPair.Template.Network.PrivateSubnet.Id = newId
// 	prjPair.Live.Network.PrivateSubnet.Id = newId
// }

// func (prjPair *ProjectPair) SetPublicSubnetId(newId string) {
// 	prjPair.Template.Network.PublicSubnet.Id = newId
// 	prjPair.Live.Network.PublicSubnet.Id = newId
// }

// func (prjPair *ProjectPair) SetVolumeId(iNickname string, volNickname string, newId string) {
// 	prjPair.Template.Instances[iNickname].Volumes[volNickname].VolumeId = newId
// 	prjPair.Live.Instances[iNickname].Volumes[volNickname].VolumeId = newId
// }

// func (prjPair *ProjectPair) SetAttachedVolumeDevice(iNickname string, volNickname string, device string) {
// 	prjPair.Template.Instances[iNickname].Volumes[volNickname].Device = device
// 	prjPair.Live.Instances[iNickname].Volumes[volNickname].Device = device
// }

// func (prjPair *ProjectPair) SetVolumeBlockDeviceId(iNickname string, volNickname string, newId string) {
// 	prjPair.Template.Instances[iNickname].Volumes[volNickname].BlockDeviceId = newId
// 	prjPair.Live.Instances[iNickname].Volumes[volNickname].BlockDeviceId = newId
// }

// func (prjPair *ProjectPair) CleanInstance(iNickname string) {
// 	prjPair.Template.Instances[iNickname].Clean()
// 	prjPair.Live.Instances[iNickname].Clean()
// }

// func (prjPair *ProjectPair) SetInstanceId(iNickname string, newId string) {
// 	prjPair.Template.Instances[iNickname].Id = newId
// 	prjPair.Live.Instances[iNickname].Id = newId
// }

// func (prjPair *ProjectPair) SetInstanceSnapshotImageId(iNickname string, newId string) {
// 	prjPair.Template.Instances[iNickname].SnapshotImageId = newId
// 	prjPair.Live.Instances[iNickname].SnapshotImageId = newId
// }

func (prj *Project) validate() error {
	// Check instance presence and uniqueness: hostnames, ip addresses, security groups
	hostnameMap := map[string]struct{}{}
	internalIpMap := map[string]struct{}{}
	bastionExternalIpInstanceNickname := ""
	for iNickname, iDef := range prj.Instances {
		if iDef.InstName == "" {
			return fmt.Errorf("instance %s has empty Instname", iNickname)
		}
		if _, ok := hostnameMap[iDef.InstName]; ok {
			return fmt.Errorf("instances share Instname %s", iDef.InstName)
		}
		hostnameMap[iDef.InstName] = struct{}{}

		if iDef.IpAddress == "" {
			return fmt.Errorf("instance %s has empty ip address", iNickname)
		}
		if _, ok := internalIpMap[iDef.IpAddress]; ok {
			return fmt.Errorf("instances share internal ip %s", iDef.IpAddress)
		}
		internalIpMap[iDef.IpAddress] = struct{}{}

		if iDef.ExternalIpAddressName != "" {
			if iDef.ExternalIpAddressName != prj.SshConfig.BastionExternalIpAddressName {
				return fmt.Errorf("instance %s has unexpeted external ip name %s, expected %s", iNickname, iDef.ExternalIpAddressName, prj.SshConfig.BastionExternalIpAddressName)
			}
			if bastionExternalIpInstanceNickname != "" {
				return fmt.Errorf("instances %s,%s share external ip address %s", iNickname, bastionExternalIpInstanceNickname, prj.SshConfig.BastionExternalIpAddressName)
			}
			bastionExternalIpInstanceNickname = iNickname
		}

		// Security groups
		if iDef.SecurityGroupName == "" {
			return fmt.Errorf("instance %s has empty security group name", iNickname)
		}

		sgFound := false
		for _, sgDef := range prj.SecurityGroups {
			if sgDef.Name == iDef.SecurityGroupName {
				sgFound = true
				break
			}
		}
		if !sgFound {
			return fmt.Errorf("instance %s has invalid security group %s", iNickname, iDef.SecurityGroupName)
		}

		// External ip address

		if iDef.ExternalIpAddressName != "" {
			if iDef.ExternalIpAddressName != prj.SshConfig.BastionExternalIpAddressName && iDef.ExternalIpAddressName != prj.Network.PublicSubnet.NatGatewayExternalIpName {
				return fmt.Errorf("instance %s has invalid external ip address name %s, expected %s or %s ", iNickname, iDef.SecurityGroupName, prj.SshConfig.BastionExternalIpAddressName, prj.Network.PublicSubnet.NatGatewayExternalIpName)
			}
		}
	}

	// Need at least one floating ip address
	if bastionExternalIpInstanceNickname == "" {
		return fmt.Errorf("none of the instances is using ssh_config_external_ip, at least one must have it")
	}

	scriptsMap := map[string]bool{}
	if err := rexec.HarvestAllEmbeddedFilesPaths("", scriptsMap); err != nil {
		return err
	}
	missingScriptsMap := map[string]struct{}{}
	for _, iDef := range prj.Instances {
		allInstanceScripts := append(append(append(iDef.Service.Cmd.Install, iDef.Service.Cmd.Config...), iDef.Service.Cmd.Start...), iDef.Service.Cmd.Stop...)
		for _, scriptPath := range allInstanceScripts {
			if _, ok := scriptsMap[scriptPath]; !ok {
				missingScriptsMap[scriptPath] = struct{}{}
			} else {
				scriptsMap[scriptPath] = true
			}
		}
	}

	// Verify that all scripts mentioned in the project are present
	if len(missingScriptsMap) > 0 {
		missingScripts := make([]string, len(missingScriptsMap))
		i := 0
		for scriptPath, _ := range missingScriptsMap {
			missingScripts[i] = scriptPath
			i++
		}
		return fmt.Errorf("cannot find embedded script(s): %s", strings.Join(missingScripts, ","))
	}

	// Vice versa: verify all existing scripts are used
	unusedScripts := make([]string, 0)
	i := 0
	for scriptPath, isUsed := range scriptsMap {
		if !isUsed {
			unusedScripts = append(unusedScripts, scriptPath)
		}
		i++
	}
	if len(unusedScripts) > 0 {
		return fmt.Errorf("the following embedded scripts are not used in this project: %s", strings.Join(unusedScripts, ","))
	}

	return nil
}

func LoadProject(prjFile string) (*Project, error) {
	prjFullPath, err := filepath.Abs(prjFile)
	if err != nil {
		return nil, fmt.Errorf("cannot get absolute path of %s: %s", prjFile, err.Error())
	}

	if _, err := os.Stat(prjFullPath); err != nil {
		return nil, fmt.Errorf("cannot find project file [%s]: [%s]", prjFullPath, err.Error())
	}

	vm := jsonnet.MakeVM()
	prjString, err := vm.EvaluateFile(prjFile)
	if err != nil {
		return nil, err
	}

	// prjBytes, err := os.ReadFile(prjFullPath)
	// if err != nil {
	// 	return nil, "", fmt.Errorf("cannot read project file %s: %s", prjFullPath, err.Error())
	// }

	//prjPair := ProjectPair{}

	// Read project

	// err = json.Unmarshal(prjBytes, &prjPair.Template)
	// if err != nil {
	// 	return nil, "", fmt.Errorf("cannot parse project file %s: %s", prjFullPath, err.Error())
	// }

	// prjString := string(prjBytes)

	envVars := map[string]string{}
	missingVars := make([]string, 0)
	r := regexp.MustCompile(`\{(CAPIDEPLOY[_A-Z0-9]+)\}`)
	matches := r.FindAllStringSubmatch(prjString, -1)
	for _, v := range matches {
		envVar := v[1]
		envVars[envVar] = os.Getenv(envVar)
		if envVars[envVar] == "" {
			missingVars = append(missingVars, envVar)
		}
	}

	if len(missingVars) > 0 {
		return nil, fmt.Errorf("cannot load deployment project, missing env variables:\n%v", strings.Join(missingVars, "\n"))
	}

	// Replace env vars

	// Revert unescaping in parameter values caused by JSON - we want to preserve `\n"` and `\"`
	escapeReplacer := strings.NewReplacer("\n", "\\n", `"`, `\"`)
	for k, v := range envVars {
		prjString = strings.ReplaceAll(prjString, fmt.Sprintf("{%s}", k), escapeReplacer.Replace(v))
	}

	// Hacky way to provide bastion ip
	// prjString = strings.ReplaceAll(prjString, "{CAPIDEPLOY.INTERNAL.BASTION_EXTERNAL_IP_ADDRESS}", prjPair.Template.SshConfig.BastionExternalIp)

	// Re-deserialize forom prjString, now with replaced params

	project := Project{}
	if err := json.Unmarshal([]byte(prjString), &project); err != nil {
		return nil, fmt.Errorf("cannot parse project file with replaced vars %s: %s", prjFullPath, err.Error())
	}

	if project.DeployProviderName != DeployProviderAws {
		return nil, fmt.Errorf("cannot parse deploy provider name %s, expected [%s]",
			project.DeployProviderName,
			DeployProviderAws)
	}

	// Defaults

	project.InitDefaults()

	if err := project.validate(); err != nil {
		return nil, fmt.Errorf("cannot load project file %s: %s", prjFullPath, err.Error())
	}

	return &project, nil
}

// func (prj *Project) SaveProject(fullPrjPath string) error {
// 	prjJsonBytes, err := json.MarshalIndent(prj, "", "    ")
// 	if err != nil {
// 		return err
// 	}

// 	fPrj, err := os.Create(fullPrjPath)
// 	if err != nil {
// 		return err
// 	}
// 	defer fPrj.Close()
// 	if _, err := fPrj.WriteString(string(prjJsonBytes)); err != nil {
// 		return err
// 	}
// 	return fPrj.Sync()
// }
