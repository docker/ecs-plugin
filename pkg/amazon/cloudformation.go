package amazon

import (
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"

	cloudmapapi "github.com/aws/aws-sdk-go/service/servicediscovery"

	ecsapi "github.com/aws/aws-sdk-go/service/ecs"
	"github.com/awslabs/goformation/v4/cloudformation"
	"github.com/awslabs/goformation/v4/cloudformation/ec2"
	"github.com/awslabs/goformation/v4/cloudformation/ecs"
	"github.com/awslabs/goformation/v4/cloudformation/iam"
	"github.com/awslabs/goformation/v4/cloudformation/logs"
	cloudmap "github.com/awslabs/goformation/v4/cloudformation/servicediscovery"
	"github.com/awslabs/goformation/v4/cloudformation/tags"
	"github.com/docker/ecs-plugin/pkg/compose"
)

const (
	ParameterClusterName = "ParameterClusterName"
	ParameterVPCId       = "ParameterVPCId"
	ParameterSubnet1Id   = "ParameterSubnet1Id"
	ParameterSubnet2Id   = "ParameterSubnet2Id"
)

// Convert a compose project into a CloudFormation template
func (c client) Convert(project *compose.Project) (*cloudformation.Template, error) {
	warnings := Check(project)
	for _, w := range warnings {
		logrus.Warn(w)
	}

	template := cloudformation.NewTemplate()

	template.Parameters[ParameterClusterName] = cloudformation.Parameter{
		Type:        "String",
		Description: "Name of the ECS cluster to deploy to (optional)",
	}

	template.Parameters[ParameterVPCId] = cloudformation.Parameter{
		Type:        "AWS::EC2::VPC::Id",
		Description: "ID of the VPC",
	}

	/*
		FIXME can't set subnets: Ref("SubnetIds") see https://github.com/awslabs/goformation/issues/282
		template.Parameters["SubnetIds"] = cloudformation.Parameter{
			Type:        "List<AWS::EC2::Subnet::Id>",
			Description: "The list of SubnetIds, for at least two Availability Zones in the region in your VPC",
		}
	*/
	template.Parameters[ParameterSubnet1Id] = cloudformation.Parameter{
		Type:        "AWS::EC2::Subnet::Id",
		Description: "SubnetId,for Availability Zone 1 in the region in your VPC",
	}
	template.Parameters[ParameterSubnet2Id] = cloudformation.Parameter{
		Type:        "AWS::EC2::Subnet::Id",
		Description: "SubnetId,for Availability Zone 1 in the region in your VPC",
	}

	// Create Cluster is `ParameterClusterName` parameter is not set
	template.Conditions["CreateCluster"] = cloudformation.Equals("", cloudformation.Ref(ParameterClusterName))

	template.Resources["Cluster"] = &ecs.Cluster{
		ClusterName: project.Name,
		Tags: []tags.Tag{
			{
				Key:   ProjectTag,
				Value: project.Name,
			},
		},
		AWSCloudFormationCondition: "CreateCluster",
	}
	cluster := cloudformation.If("CreateCluster", cloudformation.Ref("Cluster"), cloudformation.Ref(ParameterClusterName))

	for net := range project.Networks {
		name, resource := convertNetwork(project, net, cloudformation.Ref(ParameterVPCId))
		template.Resources[name] = resource
	}

	logGroup := fmt.Sprintf("/docker-compose/%s", project.Name)
	template.Resources["LogGroup"] = &logs.LogGroup{
		LogGroupName: logGroup,
	}

	// Private DNS namespace will allow DNS name for the services to be <service>.<project>.local
	template.Resources["CloudMap"] = &cloudmap.PrivateDnsNamespace{
		Description: fmt.Sprintf("Service Map for Docker Compose project %s", project.Name),
		Name:        fmt.Sprintf("%s.local", project.Name),
		Vpc:         cloudformation.Ref(ParameterVPCId),
	}

	for _, service := range project.Services {
		definition, err := Convert(project, service)
		if err != nil {
			return nil, err
		}

		taskExecutionRole := fmt.Sprintf("%sTaskExecutionRole", service.Name)
		policy, err := c.getPolicy(definition)
		if err != nil {
			return nil, err
		}
		rolePolicies := []iam.Role_Policy{}
		if policy != nil {
			rolePolicies = append(rolePolicies, iam.Role_Policy{
				PolicyDocument: policy,
				PolicyName:     fmt.Sprintf("%sGrantAccessToSecrets", service.Name),
			})

		}
		definition.ExecutionRoleArn = cloudformation.Ref(taskExecutionRole)

		taskDefinition := fmt.Sprintf("%sTaskDefinition", service.Name)
		template.Resources[taskExecutionRole] = &iam.Role{
			AssumeRolePolicyDocument: assumeRolePolicyDocument,
			Policies:                 rolePolicies,
			ManagedPolicyArns: []string{
				ECSTaskExecutionPolicy,
				ECRReadOnlyPolicy,
			},
		}
		template.Resources[taskDefinition] = definition

		var healthCheck *cloudmap.Service_HealthCheckConfig
		if service.HealthCheck != nil && !service.HealthCheck.Disable {
			// FIXME ECS only support HTTP(s) health checks, while Docker only support CMD
		}

		serviceRegistration := fmt.Sprintf("%sServiceDiscoveryEntry", service.Name)
		template.Resources[serviceRegistration] = &cloudmap.Service{
			Description:       fmt.Sprintf("%q service discovery entry in Cloud Map", service.Name),
			HealthCheckConfig: healthCheck,
			Name:              service.Name,
			NamespaceId:       cloudformation.Ref("CloudMap"),
			DnsConfig: &cloudmap.Service_DnsConfig{
				DnsRecords: []cloudmap.Service_DnsRecord{
					{
						TTL:  300,
						Type: cloudmapapi.RecordTypeA,
					},
				},
				RoutingPolicy: cloudmapapi.RoutingPolicyMultivalue,
			},
		}

		serviceSecurityGroups := []string{}
		for net := range service.Networks {
			logicalName := networkResourceName(project, net)
			serviceSecurityGroups = append(serviceSecurityGroups, cloudformation.Ref(logicalName))
		}

		template.Resources[fmt.Sprintf("%sService", service.Name)] = &ecs.Service{
			Cluster:      cluster,
			DesiredCount: 1,
			LaunchType:   ecsapi.LaunchTypeFargate,
			NetworkConfiguration: &ecs.Service_NetworkConfiguration{
				AwsvpcConfiguration: &ecs.Service_AwsVpcConfiguration{
					AssignPublicIp: ecsapi.AssignPublicIpEnabled,
					SecurityGroups: serviceSecurityGroups,
					Subnets: []string{
						cloudformation.Ref(ParameterSubnet1Id),
						cloudformation.Ref(ParameterSubnet2Id),
					},
				},
			},
			SchedulingStrategy: ecsapi.SchedulingStrategyReplica,
			ServiceName:        service.Name,
			ServiceRegistries: []ecs.Service_ServiceRegistry{
				{
					RegistryArn: cloudformation.GetAtt(serviceRegistration, "Arn"),
				},
			},
			TaskDefinition: cloudformation.Ref(taskDefinition),
		}
	}
	return template, nil
}

func convertNetwork(project *compose.Project, net string, vpc string) (string, cloudformation.Resource) {
	var ingresses []ec2.SecurityGroup_Ingress
	for _, service := range project.Services {
		if _, ok := service.Networks[net]; ok {
			for _, port := range service.Ports {
				ingresses = append(ingresses, ec2.SecurityGroup_Ingress{
					CidrIp:      "0.0.0.0/0",
					Description: fmt.Sprintf("%s:%d/%s", service.Name, port.Target, port.Protocol),
					FromPort:    int(port.Target),
					IpProtocol:  strings.ToUpper(port.Protocol),
					ToPort:      int(port.Target),
				})
			}
		}
	}

	securityGroup := networkResourceName(project, net)
	resource := &ec2.SecurityGroup{
		GroupDescription:     fmt.Sprintf("%s %s Security Group", project.Name, net),
		GroupName:            securityGroup,
		SecurityGroupIngress: ingresses,
		VpcId:                vpc,
		Tags: []tags.Tag{
			{
				Key:   ProjectTag,
				Value: project.Name,
			},
			{
				Key:   NetworkTag,
				Value: net,
			},
		},
	}
	return securityGroup, resource
}

func networkResourceName(project *compose.Project, network string) string {
	return fmt.Sprintf("%s%sNetwork", project.Name, strings.Title(network))
}

func (c client) getPolicy(taskDef *ecs.TaskDefinition) (*PolicyDocument, error) {

	arns := []string{}
	for _, container := range taskDef.ContainerDefinitions {
		if container.RepositoryCredentials != nil {
			arns = append(arns, container.RepositoryCredentials.CredentialsParameter)
		}
		if len(container.Secrets) > 0 {
			for _, s := range container.Secrets {
				arns = append(arns, s.ValueFrom)
			}
		}

	}
	if len(arns) > 0 {
		return &PolicyDocument{
			Statement: []PolicyStatement{
				{
					Effect:   "Allow",
					Action:   []string{ActionGetSecretValue, ActionGetParameters, ActionDecrypt},
					Resource: arns,
				}},
		}, nil
	}
	return nil, nil
}
