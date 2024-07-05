package cld

import "fmt"

type ResourceBilledState string

const (
	ResourceBilledStateUnknown    ResourceBilledState = "unknown"
	ResourceBilledStateActive     ResourceBilledState = "active"
	ResourceBilledStateTerminated ResourceBilledState = "terminated"
)

const DeploymentNameTagName string = "DeploymentName"
const DeploymentOperatorTagName string = "DeploymentOperator"
const DeploymentOperatorTagValue string = "capideploy"

type Resource struct {
	DeploymentName string
	Svc            string
	Type           string
	Id             string
	Name           string
	State          string
	BilledState    ResourceBilledState
}

func (r *Resource) String() string {
	return fmt.Sprintf("%s, %s,%s,%s,%s,%s,%s", r.DeploymentName, r.Svc, r.Type, r.Name, r.Id, r.State, r.BilledState)
}
