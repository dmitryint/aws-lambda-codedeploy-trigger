package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/dmitryint/aws-lambda-codedeploy-trigger/pkg/deploy"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/codedeploy"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
)

const (
	defaultPollInterval = 15 * time.Second
	pollIntervalEnv     = "POLL_INTERVAL_SECONDS"
	deadlineMargin      = 5 * time.Second
)

// revision is overridden at link time via -ldflags "-X main.revision=...".
var revision = "dev"

func pollInterval() time.Duration {
	raw, ok := os.LookupEnv(pollIntervalEnv)
	if !ok {
		return defaultPollInterval
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return defaultPollInterval
	}
	return time.Duration(n) * time.Second
}

type handler struct {
	codeDeploy   codeDeployAPI
	ecs          ecsAPI
	pollInterval time.Duration
}

func (h handler) handleRequest(ctx context.Context, event deploy.Event) (deploy.Result, error) {
	trigger, err := event.Trigger()
	if err != nil {
		return deploy.Result{}, err
	}

	if event.ShouldWait() {
		var cancel context.CancelFunc
		ctx, cancel = withDeadlineMargin(ctx)
		defer cancel()
	}

	if trigger == deploy.TriggerECS {
		return runECS(ctx, h.ecs, event, h.pollInterval)
	}
	return runCodeDeploy(ctx, h.codeDeploy, event, h.pollInterval)
}

func withDeadlineMargin(ctx context.Context) (context.Context, context.CancelFunc) {
	deadline, ok := ctx.Deadline()
	if !ok || time.Until(deadline) <= deadlineMargin {
		return context.WithCancel(ctx)
	}
	return context.WithDeadline(ctx, deadline.Add(-deadlineMargin))
}

func main() {
	log.Printf("aws-lambda-codedeploy-trigger %s", revision)

	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("load AWS SDK config: %v", err)
	}

	h := handler{
		codeDeploy:   codedeploy.NewFromConfig(cfg),
		ecs:          ecs.NewFromConfig(cfg),
		pollInterval: pollInterval(),
	}
	lambda.Start(h.handleRequest)
}
