build:
	go build -v -o dist/docker-ecs cmd/main/main.go

test: ## Run tests
	go test ./... -v

dev: build
	ln -f -s "${PWD}/dist/docker-ecs" "${HOME}/.docker/cli-plugins/docker-ecs"

mock:
	mockgen --package mock github.com/aws/aws-sdk-go/service/ec2/ec2iface EC2API > pkg/amazon/mock/ec2.go
	mockgen --package mock github.com/aws/aws-sdk-go/service/ecs/ecsiface ECSAPI > pkg/amazon/mock/ecs.go
	mockgen --package mock github.com/aws/aws-sdk-go/service/elbv2/elbv2iface ELBV2API > pkg/amazon/mock/elb.go
	mockgen --package mock github.com/aws/aws-sdk-go/service/cloudwatchlogs/cloudwatchlogsiface CloudWatchLogsAPI > pkg/amazon/mock/cloudwatchlogs.go
	mockgen --package mock github.com/aws/aws-sdk-go/service/iam/iamiface IAMAPI > pkg/amazon/mock/iam.go
	mockgen --package mock github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface CloudFormationAPI > pkg/amazon/mock/cloudformation.go

.PHONY: build test dev