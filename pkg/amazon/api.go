package amazon

import "context"

//go:generate mockgen -destination=./mock/api.go -package=mock . API

type API interface {
	downAPI
	upAPI
	logsAPI
	secretsAPI
	GetTasks(ctx context.Context, cluster string, name string) ([]string, error)
	GetNetworkInterfaces(ctx context.Context, cluster string, arns ...string) ([]string, error)
	GetPublicIPs(ctx context.Context, interfaces ...string) ([]string, error)
}
