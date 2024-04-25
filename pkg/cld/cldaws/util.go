package cldaws

import (
	"context"
	"fmt"
	"runtime"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/capillariesio/capillaries-deploy/pkg/l"
)

func CurAwsFuncName() string {
	pc := make([]uintptr, 15)
	n := runtime.Callers(2, pc)
	frames := runtime.CallersFrames(pc[:n])
	frame, _ := frames.Next()
	return "aws." + frame.Function
}

func TagResourceName(client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, resourceId string, tagName string) error {
	out, err := client.CreateTags(goCtx, &ec2.CreateTagsInput{
		Resources: []string{resourceId},
		Tags:      []types.Tag{{Key: aws.String("Name"), Value: aws.String(tagName)}}})
	lb.AddObject(out)
	if err != nil {
		return fmt.Errorf("cannot tag resource %s as %s: %s", resourceId, tagName, err.Error())
	}
	return nil
}
