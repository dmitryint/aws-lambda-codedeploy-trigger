package main

import (
	"context"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/codedeploy"
	cdtypes "github.com/aws/aws-sdk-go-v2/service/codedeploy/types"
)

type fakeCodeDeploy struct {
	deploymentID    string
	statuses        []cdtypes.DeploymentStatus
	errorMessage    string
	createErr       error
	getErr          error
	blockAfterCalls int

	mu             sync.Mutex
	getCalls       int
	createInput    *codedeploy.CreateDeploymentInput
	createDeadline time.Time
}

func (f *fakeCodeDeploy) CreateDeployment(ctx context.Context, params *codedeploy.CreateDeploymentInput, _ ...func(*codedeploy.Options)) (*codedeploy.CreateDeploymentOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.createInput = params
	f.createDeadline, _ = ctx.Deadline()
	if f.createErr != nil {
		return nil, f.createErr
	}
	return &codedeploy.CreateDeploymentOutput{DeploymentId: &f.deploymentID}, nil
}

func (f *fakeCodeDeploy) GetDeployment(ctx context.Context, _ *codedeploy.GetDeploymentInput, _ ...func(*codedeploy.Options)) (*codedeploy.GetDeploymentOutput, error) {
	f.mu.Lock()
	f.getCalls++
	call := f.getCalls
	f.mu.Unlock()

	if f.blockAfterCalls > 0 && call > f.blockAfterCalls {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if f.getErr != nil {
		return nil, f.getErr
	}

	info := &cdtypes.DeploymentInfo{
		DeploymentId: &f.deploymentID,
		Status:       f.statuses[min(call-1, len(f.statuses)-1)],
	}
	if f.errorMessage != "" {
		info.ErrorInformation = &cdtypes.ErrorInformation{Message: &f.errorMessage}
	}
	return &codedeploy.GetDeploymentOutput{DeploymentInfo: info}, nil
}

func (f *fakeCodeDeploy) calls() (getCalls int, createInput *codedeploy.CreateDeploymentInput, createDeadline time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.getCalls, f.createInput, f.createDeadline
}
