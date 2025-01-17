package ecs

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/evergreen-ci/cocoa"
	"github.com/evergreen-ci/cocoa/awsutil"
	"github.com/evergreen-ci/utility"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
)

// BasicClient provides a cocoa.ECSClient implementation that wraps the AWS
// ECS API. It supports retrying requests using exponential backoff and jitter.
type BasicClient struct {
	awsutil.BaseClient
	ecs *ecs.ECS
}

// NewBasicClient creates a new AWS ECS client from the given options.
func NewBasicClient(opts awsutil.ClientOptions) (*BasicClient, error) {
	c := &BasicClient{
		BaseClient: awsutil.NewBaseClient(opts),
	}
	if err := c.setup(); err != nil {
		return nil, errors.Wrap(err, "setting up client")
	}

	return c, nil
}

func (c *BasicClient) setup() error {
	if c.ecs != nil {
		return nil
	}

	sess, err := c.GetSession()
	if err != nil {
		return errors.Wrap(err, "initializing session")
	}

	c.ecs = ecs.New(sess)

	return nil
}

// RegisterTaskDefinition registers a new task definition.
func (c *BasicClient) RegisterTaskDefinition(ctx context.Context, in *ecs.RegisterTaskDefinitionInput) (*ecs.RegisterTaskDefinitionOutput, error) {
	if err := c.setup(); err != nil {
		return nil, errors.Wrap(err, "setting up client")
	}

	var out *ecs.RegisterTaskDefinitionOutput
	var err error
	if err := utility.Retry(ctx, func() (bool, error) {
		msg := awsutil.MakeAPILogMessage("RegisterTaskDefinition", in)
		out, err = c.ecs.RegisterTaskDefinitionWithContext(ctx, in)
		if awsErr, ok := err.(awserr.Error); ok {
			grip.Debug(message.WrapError(awsErr, msg))
			if c.isNonRetryableErrorCode(awsErr.Code()) {
				return false, err
			}
		}
		return true, err
	}, c.GetRetryOptions()); err != nil {
		return nil, err
	}

	return out, nil
}

// DescribeTaskDefinition describes an existing task definition.
func (c *BasicClient) DescribeTaskDefinition(ctx context.Context, in *ecs.DescribeTaskDefinitionInput) (*ecs.DescribeTaskDefinitionOutput, error) {
	if err := c.setup(); err != nil {
		return nil, errors.Wrap(err, "setting up client")
	}

	var out *ecs.DescribeTaskDefinitionOutput
	var err error
	if err := utility.Retry(ctx, func() (bool, error) {
		msg := awsutil.MakeAPILogMessage("DescribeTaskDefinition", in)
		out, err = c.ecs.DescribeTaskDefinitionWithContext(ctx, in)
		if awsErr, ok := err.(awserr.Error); ok {
			grip.Debug(message.WrapError(awsErr, msg))
			if c.isNonRetryableErrorCode(awsErr.Code()) {
				return false, err
			}
		}
		return true, err
	}, c.GetRetryOptions()); err != nil {
		return nil, err
	}
	return out, nil
}

// ListTaskDefinitions returns the ARNs for the task definitions that match the
// input filters.
func (c *BasicClient) ListTaskDefinitions(ctx context.Context, in *ecs.ListTaskDefinitionsInput) (*ecs.ListTaskDefinitionsOutput, error) {
	if err := c.setup(); err != nil {
		return nil, errors.Wrap(err, "setting up client")
	}

	var out *ecs.ListTaskDefinitionsOutput
	var err error
	if err := utility.Retry(ctx, func() (bool, error) {
		msg := awsutil.MakeAPILogMessage("ListTaskDefinitions", in)
		out, err = c.ecs.ListTaskDefinitionsWithContext(ctx, in)
		if awsErr, ok := err.(awserr.Error); ok {
			grip.Debug(message.WrapError(awsErr, msg))
			if c.isNonRetryableErrorCode(awsErr.Code()) {
				return false, err
			}
		}
		return true, err
	}, c.GetRetryOptions()); err != nil {
		return nil, err
	}
	return out, nil
}

// DeregisterTaskDefinition deregisters an existing task definition.
func (c *BasicClient) DeregisterTaskDefinition(ctx context.Context, in *ecs.DeregisterTaskDefinitionInput) (*ecs.DeregisterTaskDefinitionOutput, error) {
	if err := c.setup(); err != nil {
		return nil, errors.Wrap(err, "setting up client")
	}

	var out *ecs.DeregisterTaskDefinitionOutput
	var err error
	if err := utility.Retry(ctx, func() (bool, error) {
		msg := awsutil.MakeAPILogMessage("DeregisterTaskDefinition", in)
		out, err = c.ecs.DeregisterTaskDefinitionWithContext(ctx, in)
		if awsErr, ok := err.(awserr.Error); ok {
			grip.Debug(message.WrapError(awsErr, msg))
			if c.isNonRetryableErrorCode(awsErr.Code()) {
				return false, err
			}
		}
		return true, err
	}, c.GetRetryOptions()); err != nil {
		return nil, err
	}

	return out, nil
}

// RunTask runs a new task.
func (c *BasicClient) RunTask(ctx context.Context, in *ecs.RunTaskInput) (*ecs.RunTaskOutput, error) {
	if err := c.setup(); err != nil {
		return nil, errors.Wrap(err, "setting up client")
	}

	var out *ecs.RunTaskOutput
	var err error
	if err := utility.Retry(ctx, func() (bool, error) {
		msg := awsutil.MakeAPILogMessage("RunTask", in)
		out, err = c.ecs.RunTaskWithContext(ctx, in)
		if awsErr, ok := err.(awserr.Error); ok {
			grip.Debug(message.WrapError(awsErr, msg))
			if strings.Contains(awsErr.Error(), "provisioning capacity limit exceeded") {
				// The ECS cluster has exceeded its maximum limit for number of
				// tasks in the PROVISIONING state. This is a service-side issue
				// and is supposed to be transient until it can free up more
				// space for PROVISIONING tasks.
				return true, err
			}
			if c.isNonRetryableErrorCode(awsErr.Code()) {
				return false, err
			}
		}
		if err != nil {
			return true, err
		}

		if utility.FromInt64Ptr(in.Count) == 1 && len(out.Tasks) == 0 && len(out.Failures) > 0 {
			// As a special case, if it's a single task that failed to run due
			// to insufficient resources, the cluster should eventually scale
			// out to provide more resources. Therefore, this should still retry
			// as it is a transient issue. This is not done for multiple tasks
			// since it may have partially succeeded in running some of them or
			// may have failed for other reasons.
			catcher := grip.NewBasicCatcher()
			for _, f := range out.Failures {
				if f == nil {
					continue
				}
				if utility.StringSliceContains([]string{"RESOURCE:CPU", "RESOURCE:MEMORY"}, utility.FromStringPtr(f.Reason)) {
					catcher.Add(ConvertFailureToError(f))
				}
			}
			return catcher.HasErrors(), errors.Wrap(catcher.Resolve(), "cluster has insufficient resources")
		}

		return false, nil
	}, c.GetRetryOptions()); err != nil {
		return nil, err
	}

	return out, nil
}

// DescribeTasks describes one or more existing tasks.
func (c *BasicClient) DescribeTasks(ctx context.Context, in *ecs.DescribeTasksInput) (*ecs.DescribeTasksOutput, error) {
	if err := c.setup(); err != nil {
		return nil, errors.Wrap(err, "setting up client")
	}

	var out *ecs.DescribeTasksOutput
	var err error
	if err := utility.Retry(ctx, func() (bool, error) {
		msg := awsutil.MakeAPILogMessage("DescribeTasks", in)
		out, err = c.ecs.DescribeTasksWithContext(ctx, in)
		if awsErr, ok := err.(awserr.Error); ok {
			grip.Debug(message.WrapError(awsErr, msg))
			if c.isNonRetryableErrorCode(awsErr.Code()) {
				return false, err
			}
		}
		return true, err
	}, c.GetRetryOptions()); err != nil {
		return nil, err
	}
	return out, nil
}

// ListTasks returns the ARNs for the task that match the input filters.
func (c *BasicClient) ListTasks(ctx context.Context, in *ecs.ListTasksInput) (*ecs.ListTasksOutput, error) {
	if err := c.setup(); err != nil {
		return nil, errors.Wrap(err, "setting up client")
	}

	var out *ecs.ListTasksOutput
	var err error
	if err := utility.Retry(ctx, func() (bool, error) {
		msg := awsutil.MakeAPILogMessage("ListTasks", in)
		out, err = c.ecs.ListTasksWithContext(ctx, in)
		if awsErr, ok := err.(awserr.Error); ok {
			grip.Debug(message.WrapError(awsErr, msg))
			if c.isNonRetryableErrorCode(awsErr.Code()) {
				return false, err
			}
		}
		return true, err
	}, c.GetRetryOptions()); err != nil {
		return nil, err
	}
	return out, nil
}

// StopTask stops a running task.
func (c *BasicClient) StopTask(ctx context.Context, in *ecs.StopTaskInput) (*ecs.StopTaskOutput, error) {
	if err := c.setup(); err != nil {
		return nil, errors.Wrap(err, "setting up client")
	}

	var out *ecs.StopTaskOutput
	var err error
	if err := utility.Retry(ctx, func() (bool, error) {
		msg := awsutil.MakeAPILogMessage("StopTask", in)
		out, err = c.ecs.StopTaskWithContext(ctx, in)
		if awsErr, ok := err.(awserr.Error); ok {
			grip.Debug(message.WrapError(awsErr, msg))
			if isTaskNotFoundError(awsErr) {
				return false, cocoa.NewECSTaskNotFoundError(utility.FromStringPtr(in.Task))
			}
			if c.isNonRetryableErrorCode(awsErr.Code()) {
				return false, err
			}
		}
		return true, err
	}, c.GetRetryOptions()); err != nil {
		return nil, err
	}
	return out, nil
}

// TagResource adds tags to an existing resource in ECS.
func (c *BasicClient) TagResource(ctx context.Context, in *ecs.TagResourceInput) (*ecs.TagResourceOutput, error) {
	if err := c.setup(); err != nil {
		return nil, errors.Wrap(err, "setting up client")
	}

	var out *ecs.TagResourceOutput
	var err error
	if err := utility.Retry(ctx, func() (bool, error) {
		msg := awsutil.MakeAPILogMessage("TagResource", in)
		out, err = c.ecs.TagResourceWithContext(ctx, in)
		if awsErr, ok := err.(awserr.Error); ok {
			grip.Debug(message.WrapError(awsErr, msg))
			if c.isNonRetryableErrorCode(awsErr.Code()) {
				return false, err
			}
		}
		return true, err
	}, c.GetRetryOptions()); err != nil {
		return nil, err
	}
	return out, nil
}

// Close cleans up all resources owned by the client.
func (c *BasicClient) Close(ctx context.Context) error {
	return c.BaseClient.Close(ctx)
}

// isNonRetryableErrorCode returns whether or not the error code from ECS is
// known to be not retryable.
func (c *BasicClient) isNonRetryableErrorCode(code string) bool {
	switch code {
	case ecs.ErrCodeAccessDeniedException,
		ecs.ErrCodeClientException,
		ecs.ErrCodeInvalidParameterException,
		ecs.ErrCodeClusterNotFoundException,
		request.InvalidParameterErrCode,
		request.ParamRequiredErrCode:
		return true
	default:
		return false
	}
}

// isTaskNotFoundError returns whether or not the error returned from ECS is
// because the task cannot be found.
func isTaskNotFoundError(err error) bool {
	awsErr, ok := err.(awserr.Error)
	if !ok {
		return false
	}
	return awsErr.Code() == ecs.ErrCodeInvalidParameterException && strings.Contains(awsErr.Message(), "The referenced task was not found")
}

// ConvertFailureToError converts an ECS failure message into a formatted error.
// If the failure is due to being unable to find the task, it will return a
// cocoa.ECSTaskNotFound error.
// Docs: https://docs.aws.amazon.com/AmazonECS/latest/developerguide/api_failures_messages.html
func ConvertFailureToError(f *ecs.Failure) error {
	if f == nil {
		return nil
	}

	if isTaskNotFoundFailure(*f) {
		return cocoa.NewECSTaskNotFoundError(utility.FromStringPtr(f.Arn))
	}
	var parts []string
	if arn := utility.FromStringPtr(f.Arn); arn != "" {
		parts = append(parts, fmt.Sprintf("task '%s'", arn))
	}
	if reason := utility.FromStringPtr(f.Reason); reason != "" {
		parts = append(parts, fmt.Sprintf("(reason) %s", reason))
	}
	if detail := utility.FromStringPtr(f.Detail); detail != "" {
		parts = append(parts, fmt.Sprintf("(detail) %s", detail))
	}
	if len(parts) == 0 {
		return errors.New("ECS failure did not contain any additional failure information")
	}
	return errors.New(strings.Join(parts, ": "))
}

// isTaskNotFoundFailure returns whether or not the failure reason returned from
// ECS is because the task cannot be found.
func isTaskNotFoundFailure(f ecs.Failure) bool {
	return f.Arn != nil && utility.FromStringPtr(f.Reason) == ReasonTaskMissing
}

// ReasonTaskMissing indicates that a task cannot be found because it is
// missing. This can happen for reasons such as the task never existed, or it
// has been stopped for a long time.
const ReasonTaskMissing = "MISSING"
