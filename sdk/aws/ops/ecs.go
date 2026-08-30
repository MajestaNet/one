package ops

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

// ECSConfig selects the install's ECS services for product rolls.
type ECSConfig struct {
	Cluster          string
	APIService       string
	WorkerService    string
	APITaskFamily    string
	WorkerTaskFamily string
	Region           string
}

// ECSConfigured reports whether enough env is present to drive ECS.
func ECSConfigured(c ECSConfig) bool {
	return c.Cluster != "" && c.APIService != "" && c.WorkerService != "" &&
		c.APITaskFamily != "" && c.WorkerTaskFamily != ""
}

// ECSAPI is the subset of the ECS client used by AWSRoller (mockable).
type ECSAPI interface {
	DescribeServices(ctx context.Context, params *ecs.DescribeServicesInput, optFns ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error)
	DescribeTaskDefinition(ctx context.Context, params *ecs.DescribeTaskDefinitionInput, optFns ...func(*ecs.Options)) (*ecs.DescribeTaskDefinitionOutput, error)
	RegisterTaskDefinition(ctx context.Context, params *ecs.RegisterTaskDefinitionInput, optFns ...func(*ecs.Options)) (*ecs.RegisterTaskDefinitionOutput, error)
	UpdateService(ctx context.Context, params *ecs.UpdateServiceInput, optFns ...func(*ecs.Options)) (*ecs.UpdateServiceOutput, error)
}

// NewAWSRoller returns an ECS roller for the given config.
// Callers should check ECSConfigured before use; MemoryRoller stays in the product tree.
func NewAWSRoller(c ECSConfig) *AWSRoller {
	return &AWSRoller{Config: c}
}

// AWSRoller drives ECS RegisterTaskDefinition + UpdateService from the API task role.
// Method set CaptureCurrent / Roll / Rollback matches product ops.Roller.
type AWSRoller struct {
	Config ECSConfig
	Client ECSAPI // optional; lazy-loaded from default AWS config when nil
}

func (*AWSRoller) Mode() string { return "ecs" }

func (r *AWSRoller) client(ctx context.Context) (ECSAPI, error) {
	if r.Client != nil {
		return r.Client, nil
	}
	opts := []func(*awsconfig.LoadOptions) error{}
	if r.Config.Region != "" {
		opts = append(opts, awsconfig.WithRegion(r.Config.Region))
	}
	cfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("aws config: %w", err)
	}
	r.Client = ecs.NewFromConfig(cfg)
	return r.Client, nil
}

func (r *AWSRoller) CaptureCurrent(ctx context.Context) (string, string, error) {
	cli, err := r.client(ctx)
	if err != nil {
		return "", "", err
	}
	api, err := r.serviceTaskDef(ctx, cli, r.Config.APIService)
	if err != nil {
		return "", "", err
	}
	worker, err := r.serviceTaskDef(ctx, cli, r.Config.WorkerService)
	if err != nil {
		return "", "", err
	}
	return api, worker, nil
}

func (r *AWSRoller) serviceTaskDef(ctx context.Context, cli ECSAPI, service string) (string, error) {
	out, err := cli.DescribeServices(ctx, &ecs.DescribeServicesInput{
		Cluster:  aws.String(r.Config.Cluster),
		Services: []string{service},
	})
	if err != nil {
		return "", err
	}
	if len(out.Services) == 0 || out.Services[0].TaskDefinition == nil {
		return "", fmt.Errorf("service %s not found", service)
	}
	return aws.ToString(out.Services[0].TaskDefinition), nil
}

func (r *AWSRoller) Roll(ctx context.Context, req RollRequest) (string, string, error) {
	cli, err := r.client(ctx)
	if err != nil {
		return "", "", err
	}
	apiARN, err := r.registerImage(ctx, cli, r.Config.APITaskFamily, req.APIImage, req.ProductVersion)
	if err != nil {
		return "", "", err
	}
	workerARN, err := r.registerImage(ctx, cli, r.Config.WorkerTaskFamily, req.WorkerImage, req.ProductVersion)
	if err != nil {
		return "", "", err
	}
	if _, err := cli.UpdateService(ctx, &ecs.UpdateServiceInput{
		Cluster:            aws.String(r.Config.Cluster),
		Service:            aws.String(r.Config.APIService),
		TaskDefinition:     aws.String(apiARN),
		ForceNewDeployment: true,
	}); err != nil {
		return "", "", fmt.Errorf("update api service: %w", err)
	}
	if _, err := cli.UpdateService(ctx, &ecs.UpdateServiceInput{
		Cluster:            aws.String(r.Config.Cluster),
		Service:            aws.String(r.Config.WorkerService),
		TaskDefinition:     aws.String(workerARN),
		ForceNewDeployment: true,
	}); err != nil {
		return "", "", fmt.Errorf("update worker service: %w", err)
	}
	return apiARN, workerARN, nil
}

func (r *AWSRoller) registerImage(ctx context.Context, cli ECSAPI, family, image, productVersion string) (string, error) {
	desc, err := cli.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: aws.String(family),
	})
	if err != nil {
		return "", fmt.Errorf("describe task def %s: %w", family, err)
	}
	td := desc.TaskDefinition
	containers := make([]types.ContainerDefinition, 0, len(td.ContainerDefinitions))
	for _, c := range td.ContainerDefinitions {
		cp := c
		cp.Image = aws.String(image)
		env := make([]types.KeyValuePair, 0, len(cp.Environment)+1)
		found := false
		for _, e := range cp.Environment {
			if aws.ToString(e.Name) == "PRODUCT_VERSION" {
				env = append(env, types.KeyValuePair{Name: aws.String("PRODUCT_VERSION"), Value: aws.String(productVersion)})
				found = true
				continue
			}
			env = append(env, e)
		}
		if !found {
			env = append(env, types.KeyValuePair{Name: aws.String("PRODUCT_VERSION"), Value: aws.String(productVersion)})
		}
		cp.Environment = env
		containers = append(containers, cp)
	}
	reg, err := cli.RegisterTaskDefinition(ctx, &ecs.RegisterTaskDefinitionInput{
		Family:                  td.Family,
		TaskRoleArn:             td.TaskRoleArn,
		ExecutionRoleArn:        td.ExecutionRoleArn,
		NetworkMode:             td.NetworkMode,
		ContainerDefinitions:    containers,
		RequiresCompatibilities: td.RequiresCompatibilities,
		Cpu:                     td.Cpu,
		Memory:                  td.Memory,
		Volumes:                 td.Volumes,
		RuntimePlatform:         td.RuntimePlatform,
		EphemeralStorage:        td.EphemeralStorage,
		ProxyConfiguration:      td.ProxyConfiguration,
	})
	if err != nil {
		return "", fmt.Errorf("register task def %s: %w", family, err)
	}
	return aws.ToString(reg.TaskDefinition.TaskDefinitionArn), nil
}

func (r *AWSRoller) Rollback(ctx context.Context, apiTaskDef, workerTaskDef string) error {
	cli, err := r.client(ctx)
	if err != nil {
		return err
	}
	if apiTaskDef != "" {
		if _, err := cli.UpdateService(ctx, &ecs.UpdateServiceInput{
			Cluster:            aws.String(r.Config.Cluster),
			Service:            aws.String(r.Config.APIService),
			TaskDefinition:     aws.String(apiTaskDef),
			ForceNewDeployment: true,
		}); err != nil {
			return fmt.Errorf("rollback api: %w", err)
		}
	}
	if workerTaskDef != "" {
		if _, err := cli.UpdateService(ctx, &ecs.UpdateServiceInput{
			Cluster:            aws.String(r.Config.Cluster),
			Service:            aws.String(r.Config.WorkerService),
			TaskDefinition:     aws.String(workerTaskDef),
			ForceNewDeployment: true,
		}); err != nil {
			return fmt.Errorf("rollback worker: %w", err)
		}
	}
	return nil
}

// WaitStable polls DescribeServices until each service has a single primary deployment
// and runningCount >= desiredCount (or timeout).
func (r *AWSRoller) WaitStable(ctx context.Context) error {
	cli, err := r.client(ctx)
	if err != nil {
		return err
	}
	deadline := time.Now().Add(15 * time.Minute)
	services := []string{r.Config.APIService, r.Config.WorkerService}
	for time.Now().Before(deadline) {
		out, err := cli.DescribeServices(ctx, &ecs.DescribeServicesInput{
			Cluster:  aws.String(r.Config.Cluster),
			Services: services,
		})
		if err != nil {
			return err
		}
		ok := len(out.Services) == len(services)
		for _, s := range out.Services {
			if len(s.Deployments) != 1 || s.RunningCount < s.DesiredCount {
				ok = false
				break
			}
		}
		if ok {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(15 * time.Second):
		}
	}
	return fmt.Errorf("timed out waiting for ECS services to stabilize")
}
