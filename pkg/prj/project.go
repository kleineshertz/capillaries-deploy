package prj

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/capillariesio/capillaries-deploy/pkg/rexec"
)

type InstancePurpose string

const (
	InstancePurposeBastion    InstancePurpose = "bastion"
	InstancePurposeCassandra  InstancePurpose = "cassandra"
	InstancePurposeDaemon     InstancePurpose = "daemon"
	InstancePurposeRabbitmq   InstancePurpose = "rabbitmq"
	InstancePurposePrometheus InstancePurpose = "prometheus"
)

type ExecTimeouts struct {
	CreateInstance   int `json:"create_instance"`
	DeleteInstance   int `json:"delete_instance"`
	CreateNatGateway int `json:"create_nat_gateway"`
	DeleteNatGateway int `json:"delete_nat_gateway"`
	CreateNetwork    int `json:"create_network"`
	AttachVolume     int `json:"attach_volume"`
}

func (t *ExecTimeouts) InitDefaults() {
	if t.CreateInstance == 0 {
		t.CreateInstance = 120
	}
	if t.DeleteInstance == 0 {
		t.DeleteInstance = 120
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
}

type SecurityGroupRuleDef struct {
	Desc      string `json:"desc"`      // human-readable
	Id        string `json:"id"`        // guid
	Protocol  string `json:"protocol"`  // tcp
	Ethertype string `json:"ethertype"` // IPv4
	RemoteIp  string `json:"remote_ip"` // 0.0.0.0/0
	Port      int    `json:"port"`      // 22
	Direction string `json:"direction"` // ingress
}

type SecurityGroupDef struct {
	Name  string                  `json:"name"`
	Id    string                  `json:"id"`
	Rules []*SecurityGroupRuleDef `json:"rules"`
}

func (sg *SecurityGroupDef) Clean() {
	sg.Id = ""
	for _, r := range sg.Rules {
		r.Id = ""
	}
}

type PrivateSubnetDef struct {
	Name             string `json:"name"`
	Id               string `json:"id"`
	Cidr             string `json:"cidr"`
	AvailabilityZone string `json:"availability_zone"`  // AWS only
	RouteTableToNat  string `json:"route_table_to_nat"` // AWS only
}

// AWS-specific
type PublicSubnetDef struct {
	Name               string `json:"name"`
	Id                 string `json:"id"`
	Cidr               string `json:"cidr"`
	AvailabilityZone   string `json:"availability_zone"`
	NatGatewayName     string `json:"nat_gateway_name"`
	NatGatewayId       string `json:"nat_gateway_id"`
	NatGatewayPublicIp string `json:"nat_gateway_public_ip"`
}

type RouterDef struct {
	Name string `json:"name"`
	Id   string `json:"id"`
}

type NetworkDef struct {
	Name          string           `json:"name"`
	Id            string           `json:"id"`
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
	VolumeId         string `json:"id"`
	AttachmentId     string `json:"attachment_id"`
	Device           string `json:"device"`
	BlockDeviceId    string `json:"block_device_id"`
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
	Purpose                        string                `json:"purpose"`
	InstName                       string                `json:"inst_name"`
	SecurityGroupNickname          string                `json:"security_group"`
	RootKeyName                    string                `json:"root_key_name"`
	IpAddress                      string                `json:"ip_address"`
	UsesSshConfigExternalIpAddress bool                  `json:"uses_ssh_config_external_ip_address,omitempty"`
	ExternalIpAddress              string                `json:"external_ip_address,omitempty"`
	FlavorName                     string                `json:"flavor"`
	ImageName                      string                `json:"image"`
	SubnetType                     string                `json:"subnet_type"`
	Volumes                        map[string]*VolumeDef `json:"volumes,omitempty"`
	Id                             string                `json:"id"`
	Service                        ServiceDef            `json:"service"`
}

func (iDef *InstanceDef) BestIpAddress() string {
	if iDef.ExternalIpAddress != "" {
		return iDef.ExternalIpAddress
	}
	return iDef.IpAddress
}

func (iDef *InstanceDef) Clean() {
	iDef.Id = ""
	for _, volAttachDef := range iDef.Volumes {
		volAttachDef.AttachmentId = ""
		volAttachDef.Device = ""
		volAttachDef.BlockDeviceId = ""
		// Do not clean volAttachDef.VolumeId, it should be handled by delete_volumes
	}
}

type Project struct {
	DeploymentName     string                       `json:"deployment_name"`
	SshConfig          *rexec.SshConfigDef          `json:"ssh_config"`
	Timeouts           ExecTimeouts                 `json:"timeouts"`
	EnvVariablesUsed   []string                     `json:"env_variables_used"`
	SecurityGroups     map[string]*SecurityGroupDef `json:"security_groups"`
	Network            NetworkDef                   `json:"network"`
	Instances          map[string]*InstanceDef      `json:"instances"`
	DeployProviderName string                       `json:"deploy_provider_name"`
}

func (p *Project) InitDefaults() {
	p.Timeouts.InitDefaults()
}

const DeployProviderAws string = "aws"

type ProjectPair struct {
	Template           Project
	Live               Project
	ProjectFileDirPath string
}

func (prjPair *ProjectPair) SetSecurityGroupId(sgNickname string, newId string) {
	prjPair.Template.SecurityGroups[sgNickname].Id = newId
	prjPair.Live.SecurityGroups[sgNickname].Id = newId
}

func (prjPair *ProjectPair) SetSecurityGroupRuleId(sgNickname string, ruleIdx int, newId string) {
	prjPair.Template.SecurityGroups[sgNickname].Rules[ruleIdx].Id = newId
	prjPair.Live.SecurityGroups[sgNickname].Rules[ruleIdx].Id = newId
}

func (prjPair *ProjectPair) CleanSecurityGroup(sgNickname string) {
	prjPair.Template.SecurityGroups[sgNickname].Clean()
	prjPair.Live.SecurityGroups[sgNickname].Clean()
}

func (prjPair *ProjectPair) SetNetworkId(newId string) {
	prjPair.Template.Network.Id = newId
	prjPair.Live.Network.Id = newId
}

func (prjPair *ProjectPair) SetRouterId(newId string) {
	prjPair.Template.Network.Router.Id = newId
	prjPair.Live.Network.Router.Id = newId
}

func (prjPair *ProjectPair) SetNatGatewayId(newId string) {
	prjPair.Template.Network.PublicSubnet.NatGatewayId = newId
	prjPair.Live.Network.PublicSubnet.NatGatewayId = newId
}

func (prjPair *ProjectPair) SetRouteTableToNat(newId string) {
	prjPair.Template.Network.PrivateSubnet.RouteTableToNat = newId
	prjPair.Live.Network.PrivateSubnet.RouteTableToNat = newId
}

func (prjPair *ProjectPair) SetSshExternalIp(newIp string) {
	prjPair.Template.SshConfig.ExternalIpAddress = newIp
	prjPair.Live.SshConfig.ExternalIpAddress = newIp
	for _, iDef := range prjPair.Template.Instances {
		if iDef.UsesSshConfigExternalIpAddress {
			iDef.ExternalIpAddress = newIp
		}
	}
	for _, iDef := range prjPair.Live.Instances {
		if iDef.UsesSshConfigExternalIpAddress {
			iDef.ExternalIpAddress = newIp
		}
	}
}

func (prjPair *ProjectPair) SetNatGatewayExternalIp(newIp string) {
	prjPair.Template.Network.PublicSubnet.NatGatewayPublicIp = newIp
	prjPair.Live.Network.PublicSubnet.NatGatewayPublicIp = newIp
}

func (prjPair *ProjectPair) SetPrivateSubnetId(newId string) {
	prjPair.Template.Network.PrivateSubnet.Id = newId
	prjPair.Live.Network.PrivateSubnet.Id = newId
}

func (prjPair *ProjectPair) SetPublicSubnetId(newId string) {
	prjPair.Template.Network.PublicSubnet.Id = newId
	prjPair.Live.Network.PublicSubnet.Id = newId
}

func (prjPair *ProjectPair) SetVolumeId(iNickname string, volNickname string, newId string) {
	prjPair.Template.Instances[iNickname].Volumes[volNickname].VolumeId = newId
	prjPair.Live.Instances[iNickname].Volumes[volNickname].VolumeId = newId
}

func (prjPair *ProjectPair) SetAttachedVolumeDevice(iNickname string, volNickname string, device string) {
	prjPair.Template.Instances[iNickname].Volumes[volNickname].Device = device
	prjPair.Live.Instances[iNickname].Volumes[volNickname].Device = device
}

func (prjPair *ProjectPair) SetVolumeAttachmentId(iNickname string, volNickname string, newId string) {
	prjPair.Template.Instances[iNickname].Volumes[volNickname].AttachmentId = newId
	prjPair.Live.Instances[iNickname].Volumes[volNickname].AttachmentId = newId
}

func (prjPair *ProjectPair) SetVolumeBlockDeviceId(iNickname string, volNickname string, newId string) {
	prjPair.Template.Instances[iNickname].Volumes[volNickname].BlockDeviceId = newId
	prjPair.Live.Instances[iNickname].Volumes[volNickname].BlockDeviceId = newId
}

func (prjPair *ProjectPair) CleanInstance(iNickname string) {
	prjPair.Template.Instances[iNickname].Clean()
	prjPair.Live.Instances[iNickname].Clean()
}

func (prjPair *ProjectPair) SetInstanceId(iNickname string, newId string) {
	prjPair.Template.Instances[iNickname].Id = newId
	prjPair.Live.Instances[iNickname].Id = newId
}

func (prj *Project) validate() error {
	// Check instance presence and uniqueness: hostnames, ip addresses, security groups
	hostnameMap := map[string]struct{}{}
	internalIpMap := map[string]struct{}{}
	externalIpInstanceNickname := ""
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

		if iDef.UsesSshConfigExternalIpAddress {
			if externalIpInstanceNickname != "" {
				return fmt.Errorf("instances (%s) share external ip address %s", iNickname, externalIpInstanceNickname)
			}
			externalIpInstanceNickname = iNickname
		}

		// Security groups
		if iDef.SecurityGroupNickname == "" {
			return fmt.Errorf("instance %s has empty security group", iNickname)
		}
		if _, ok := prj.SecurityGroups[iDef.SecurityGroupNickname]; !ok {
			return fmt.Errorf("instance %s has invalid security group %s", iNickname, iDef.SecurityGroupNickname)
		}
	}

	// Need at least one floating ip address
	if externalIpInstanceNickname == "" {
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

func LoadProject(prjFile string) (*ProjectPair, string, error) {
	prjFullPath, err := filepath.Abs(prjFile)
	if err != nil {
		return nil, "", fmt.Errorf("cannot get absolute path of %s: %s", prjFile, err.Error())
	}

	if _, err := os.Stat(prjFullPath); err != nil {
		return nil, "", fmt.Errorf("cannot find project file [%s]: [%s]", prjFullPath, err.Error())
	}

	prjBytes, err := os.ReadFile(prjFullPath)
	if err != nil {
		return nil, "", fmt.Errorf("cannot read project file %s: %s", prjFullPath, err.Error())
	}

	prjPair := ProjectPair{ProjectFileDirPath: filepath.Dir(prjFullPath)}

	// Read project

	err = json.Unmarshal(prjBytes, &prjPair.Template)
	if err != nil {
		return nil, "", fmt.Errorf("cannot parse project file %s: %s", prjFullPath, err.Error())
	}

	if prjPair.Template.DeployProviderName != DeployProviderAws {
		return nil, "", fmt.Errorf("cannot parse deploy provider name %s, expected [%s]",
			prjPair.Template.DeployProviderName,
			DeployProviderAws)
	}

	prjString := string(prjBytes)

	envVars := map[string]string{}
	missingVars := make([]string, 0)
	for _, envVar := range prjPair.Template.EnvVariablesUsed {
		envVars[envVar] = os.Getenv(envVar)
		if envVars[envVar] == "" {
			missingVars = append(missingVars, envVar)
		}
	}
	if len(missingVars) > 0 {
		return nil, "", fmt.Errorf("cannot load deployment project, missing env variables:\n%v", strings.Join(missingVars, "\n"))
	}

	// Replace env vars

	// Revert unescaping in parameter values caused by JSON - we want to preserve `\n"` and `\"`
	escapeReplacer := strings.NewReplacer("\n", "\\n", `"`, `\"`)
	for k, v := range envVars {
		prjString = strings.ReplaceAll(prjString, fmt.Sprintf("{%s}", k), escapeReplacer.Replace(v))
	}

	// Hacky way to provide bastion ip
	prjString = strings.ReplaceAll(prjString, "{EXTERNAL_IP_ADDRESS}", prjPair.Template.SshConfig.ExternalIpAddress)

	// Re-deserialize forom prjString, now with replaced params

	if err := json.Unmarshal([]byte(prjString), &prjPair.Live); err != nil {
		return nil, "", fmt.Errorf("cannot parse project file with replaced vars %s: %s", prjFullPath, err.Error())
	}

	// Defaults

	prjPair.Live.InitDefaults()

	if err := prjPair.Live.validate(); err != nil {
		return nil, "", fmt.Errorf("cannot load project file %s: %s", prjFullPath, err.Error())
	}

	return &prjPair, prjFullPath, nil
}

func (prj *Project) SaveProject(fullPrjPath string) error {
	prjJsonBytes, err := json.MarshalIndent(prj, "", "    ")
	if err != nil {
		return err
	}

	fPrj, err := os.Create(fullPrjPath)
	if err != nil {
		return err
	}
	defer fPrj.Close()
	if _, err := fPrj.WriteString(string(prjJsonBytes)); err != nil {
		return err
	}
	return fPrj.Sync()
}
