package backend

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/compose-spec/compose-go/cli"
	"github.com/sirupsen/logrus"

	"github.com/docker/ecs-plugin/pkg/compose"
)

// We expect tg to be of the form "<service name><protocol><port>TargetGroup"
// e.g.: "BackTCP80TargetGroup"
var targetGroupLogicalName = regexp.MustCompile("(.*)(TCP|UDP)([0-9]+)TargetGroup")

func (b *Backend) Ps(ctx context.Context, options cli.ProjectOptions) ([]compose.ServiceStatus, error) {
	projectName, err := b.projectName(options)
	if err != nil {
		return nil, err
	}
	parameters, err := b.api.ListStackParameters(ctx, projectName)
	if err != nil {
		return nil, err
	}
	loadBalancer := parameters[ParameterLoadBalancerARN]
	cluster := parameters[ParameterClusterName]

	resources, err := b.api.ListStackResources(ctx, projectName)
	if err != nil {
		return nil, err
	}

	servicesARN := []string{}
	targetGroups := []string{}
	for _, r := range resources {
		switch r.Type {
		case "AWS::ECS::Service":
			servicesARN = append(servicesARN, r.ARN)
		case "AWS::ECS::Cluster":
			cluster = r.ARN
		case "AWS::ElasticLoadBalancingV2::LoadBalancer":
			loadBalancer = r.ARN
		case "AWS::ElasticLoadBalancingV2::TargetGroup":
			targetGroups = append(targetGroups, r.LogicalID)
		}
	}

	if len(servicesARN) == 0 {
		return nil, nil
	}
	status, err := b.api.DescribeServices(ctx, cluster, servicesARN)
	if err != nil {
		return nil, err
	}

	url, err := b.api.GetLoadBalancerURL(ctx, loadBalancer)
	if err != nil {
		return nil, err
	}

	for i, state := range status {
		ports := []string{}
		for _, tg := range targetGroups {
			pb, err := parseTargetGroup(tg)
			if err != nil {
				logrus.Warn(err)
				continue
			}
			if pb.ServiceName == state.Name {
				ports = append(ports, fmt.Sprintf("%s:%s->%s/%s", url, pb.Port, pb.Port, pb.Protocol))
			}
		}
		state.Ports = ports
		status[i] = state
	}
	return status, nil
}

type portBinding struct {
	ServiceName string
	Port        string
	Protocol    string
}

func parseTargetGroup(tg string) (*portBinding, error) {
	// We expect tg to be of the form "<service name><protocol><port>TargetGroup"
	// e.g.: "BackTCP80TargetGroup"
	groups := targetGroupLogicalName.FindStringSubmatch(tg)
	// groups[0]: <service name><protocol><port>TargetGroup
	// groups[1]: <service name>
	// groups[2]: <protocol>
	// groups[3]: <port>
	if len(groups) != 4 {
		return nil, fmt.Errorf("malformed target group ID %q", tg)
	}
	pb := &portBinding{
		ServiceName: strings.ToLower(groups[1]),
		Port:        groups[3],
		Protocol:    strings.ToLower(groups[2]),
	}
	return pb, nil
}
