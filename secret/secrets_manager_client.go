package secret

import (
	"context"

	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/evergreen-ci/cocoa/awsutil"
	"github.com/evergreen-ci/utility"
)

// BasicSecretsManagerClient provides a cocoa.SecretsManagerClient
// implementation that wraps the AWS Secrets Manager API. It supports
// retrying requests using exponential backoff and jitter.
type BasicSecretsManagerClient struct {
	awsutil.BaseClient
	sm *secretsmanager.SecretsManager
}

// NewBasicSecretsManagerClient creates a new AWS Secrets Manager client from
// the given options.
func NewBasicSecretsManagerClient(opts awsutil.ClientOptions) (*BasicSecretsManagerClient, error) {
	c := &BasicSecretsManagerClient{
		BaseClient: awsutil.NewBaseClient(opts),
	}
	if err := c.setup(); err != nil {
		return nil, errors.Wrap(err, "setting up client")
	}

	return c, nil
}

func (c *BasicSecretsManagerClient) setup() error {
	if c.sm != nil {
		return nil
	}

	sess, err := c.GetSession()
	if err != nil {
		return errors.Wrap(err, "initializing session")
	}

	c.sm = secretsmanager.New(sess)

	return nil
}

// CreateSecret creates a new secret.
func (c *BasicSecretsManagerClient) CreateSecret(ctx context.Context, in *secretsmanager.CreateSecretInput) (*secretsmanager.CreateSecretOutput, error) {
	if err := c.setup(); err != nil {
		return nil, errors.Wrap(err, "setting up client")
	}

	var out *secretsmanager.CreateSecretOutput
	var err error
	if err := utility.Retry(ctx, func() (bool, error) {
		msg := awsutil.MakeAPILogMessage("CreateSecret", in)
		out, err = c.sm.CreateSecretWithContext(ctx, in)
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

// GetSecretValue gets the decrypted value of an existing secret.
func (c *BasicSecretsManagerClient) GetSecretValue(ctx context.Context, in *secretsmanager.GetSecretValueInput) (*secretsmanager.GetSecretValueOutput, error) {
	if err := c.setup(); err != nil {
		return nil, errors.Wrap(err, "setting up client")
	}

	var out *secretsmanager.GetSecretValueOutput
	var err error
	if err := utility.Retry(ctx, func() (bool, error) {
		msg := awsutil.MakeAPILogMessage("GetSecretValue", in)
		out, err = c.sm.GetSecretValueWithContext(ctx, in)
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

// DescribeSecret gets the metadata information about a secret.
func (c *BasicSecretsManagerClient) DescribeSecret(ctx context.Context, in *secretsmanager.DescribeSecretInput) (*secretsmanager.DescribeSecretOutput, error) {
	if err := c.setup(); err != nil {
		return nil, errors.Wrap(err, "setting up client")
	}

	var out *secretsmanager.DescribeSecretOutput
	var err error
	if err := utility.Retry(ctx, func() (bool, error) {
		msg := awsutil.MakeAPILogMessage("DescribeSecret", in)
		out, err = c.sm.DescribeSecretWithContext(ctx, in)
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

// ListSecrets lists the metadata information for secrets matching the filters.
func (c *BasicSecretsManagerClient) ListSecrets(ctx context.Context, in *secretsmanager.ListSecretsInput) (*secretsmanager.ListSecretsOutput, error) {
	if err := c.setup(); err != nil {
		return nil, errors.Wrap(err, "setting up client")
	}

	var out *secretsmanager.ListSecretsOutput
	var err error
	if err := utility.Retry(ctx, func() (bool, error) {
		msg := awsutil.MakeAPILogMessage("ListSecrets", in)
		out, err = c.sm.ListSecretsWithContext(ctx, in)
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

// UpdateSecretValue updates the value of an existing secret.
func (c *BasicSecretsManagerClient) UpdateSecretValue(ctx context.Context, in *secretsmanager.UpdateSecretInput) (*secretsmanager.UpdateSecretOutput, error) {
	if err := c.setup(); err != nil {
		return nil, errors.Wrap(err, "setting up client")
	}

	var out *secretsmanager.UpdateSecretOutput
	var err error
	if err := utility.Retry(ctx, func() (bool, error) {
		msg := awsutil.MakeAPILogMessage("UpdateSecret", in)
		out, err = c.sm.UpdateSecretWithContext(ctx, in)
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

// TagResource tags an existing secret.
func (c *BasicSecretsManagerClient) TagResource(ctx context.Context, in *secretsmanager.TagResourceInput) (*secretsmanager.TagResourceOutput, error) {
	if err := c.setup(); err != nil {
		return nil, errors.Wrap(err, "setting up client")
	}

	var out *secretsmanager.TagResourceOutput
	var err error
	if err := utility.Retry(ctx, func() (bool, error) {
		msg := awsutil.MakeAPILogMessage("TagResource", in)
		out, err = c.sm.TagResourceWithContext(ctx, in)
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

// DeleteSecret deletes an existing secret.
func (c *BasicSecretsManagerClient) DeleteSecret(ctx context.Context, in *secretsmanager.DeleteSecretInput) (*secretsmanager.DeleteSecretOutput, error) {
	if err := c.setup(); err != nil {
		return nil, errors.Wrap(err, "setting up client")
	}

	var out *secretsmanager.DeleteSecretOutput
	var err error
	if err := utility.Retry(ctx, func() (bool, error) {
		msg := awsutil.MakeAPILogMessage("DeleteSecret", in)
		out, err = c.sm.DeleteSecretWithContext(ctx, in)
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
func (c *BasicSecretsManagerClient) Close(ctx context.Context) error {
	return c.BaseClient.Close(ctx)
}

// isNonRetryableErrorCode returns whether or not the error code from Secrets
// Manager is known to be not retryable.
func (c *BasicSecretsManagerClient) isNonRetryableErrorCode(code string) bool {
	switch code {
	case "AccessDeniedException",
		secretsmanager.ErrCodeInvalidParameterException,
		secretsmanager.ErrCodeInvalidRequestException,
		secretsmanager.ErrCodeResourceNotFoundException,
		secretsmanager.ErrCodeResourceExistsException,
		request.InvalidParameterErrCode,
		request.ParamRequiredErrCode:
		return true
	default:
		return false
	}
}
