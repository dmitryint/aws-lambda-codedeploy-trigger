package main

import (
	"context"
	"sync"

	"github.com/aws/aws-sdk-go-v2/service/codedeploy"
	cdtypes "github.com/aws/aws-sdk-go-v2/service/codedeploy/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

type describeDeploymentResponse struct {
	deployment *types.ServiceDeployment
	failures   []types.Failure
	err        error
}

type fakeECS struct {
	mu sync.Mutex

	services       []types.Service
	serviceFailure []types.Failure
	updated        *types.Service
	updateErr      error
	deployments    []describeDeploymentResponse

	describeServicesCalls   int
	updateInputs            []*ecs.UpdateServiceInput
	describeDeploymentCalls int
}

func (f *fakeECS) DescribeServices(ctx context.Context, _ *ecs.DescribeServicesInput, _ ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.services) == 0 {
		return &ecs.DescribeServicesOutput{Failures: f.serviceFailure}, nil
	}
	s := f.services[min(f.describeServicesCalls, len(f.services)-1)]
	f.describeServicesCalls++
	return &ecs.DescribeServicesOutput{Services: []types.Service{s}}, nil
}

func (f *fakeECS) UpdateService(ctx context.Context, params *ecs.UpdateServiceInput, _ ...func(*ecs.Options)) (*ecs.UpdateServiceOutput, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updateInputs = append(f.updateInputs, params)
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	return &ecs.UpdateServiceOutput{Service: f.updated}, nil
}

func (f *fakeECS) DescribeServiceDeployments(ctx context.Context, _ *ecs.DescribeServiceDeploymentsInput, _ ...func(*ecs.Options)) (*ecs.DescribeServiceDeploymentsOutput, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.deployments) == 0 {
		return &ecs.DescribeServiceDeploymentsOutput{}, nil
	}
	r := f.deployments[min(f.describeDeploymentCalls, len(f.deployments)-1)]
	f.describeDeploymentCalls++
	if r.err != nil {
		return nil, r.err
	}
	out := &ecs.DescribeServiceDeploymentsOutput{Failures: r.failures}
	if r.deployment != nil {
		out.ServiceDeployments = []types.ServiceDeployment{*r.deployment}
	}
	return out, nil
}

type fakeCodeDeploy struct {
	mu sync.Mutex

	deploymentID string
	statuses     []cdtypes.DeploymentStatus
	getCalls     int
	createInput  *codedeploy.CreateDeploymentInput
}

func (f *fakeCodeDeploy) CreateDeployment(_ context.Context, params *codedeploy.CreateDeploymentInput, _ ...func(*codedeploy.Options)) (*codedeploy.CreateDeploymentOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.createInput = params
	return &codedeploy.CreateDeploymentOutput{DeploymentId: &f.deploymentID}, nil
}

func (f *fakeCodeDeploy) GetDeployment(_ context.Context, _ *codedeploy.GetDeploymentInput, _ ...func(*codedeploy.Options)) (*codedeploy.GetDeploymentOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	status := f.statuses[min(f.getCalls, len(f.statuses)-1)]
	f.getCalls++
	return &codedeploy.GetDeploymentOutput{DeploymentInfo: &cdtypes.DeploymentInfo{
		DeploymentId: &f.deploymentID,
		Status:       status,
	}}, nil
}
