package main

import (
	"github.com/pulumi/pulumi-aws/sdk/v2/go/aws/iam"
	"github.com/pulumi/pulumi-aws/sdk/v2/go/aws/lambda"
	"github.com/pulumi/pulumi/sdk/v2/go/pulumi"
)

func makeWebhookLambda(ctx *pulumi.Context, s3BucketID pulumi.IDOutput) (*lambda.Function, error) {
	// Create an IAM role.
	role, err := iam.NewRole(ctx, "task-exec-role", &iam.RoleArgs{
		AssumeRolePolicy: pulumi.String(`{
			"Version": "2012-10-17",
			"Statement": [{
				"Sid": "",
				"Effect": "Allow",
				"Principal": {
					"Service": "lambda.amazonaws.com"
				},
				"Action": "sts:AssumeRole"
			}]
		}`),
	})

	if err != nil {
		return nil, err
	}

	// Attach a policy to allow writing logs to CloudWatch
	logPolicy, err := iam.NewRolePolicy(ctx, "lambda-log-policy", &iam.RolePolicyArgs{
		Role: role.Name,
		Policy: pulumi.String(`{
			"Version": "2012-10-17",
			"Statement": [{
				"Effect": "Allow",
				"Action": [
					"logs:CreateLogGroup",
					"logs:CreateLogStream",
					"logs:PutLogEvents"
				],
				"Resource": "arn:aws:logs:*:*:*"
			}]
		}`),
	})

	// Attach a policy to allow writing logs to CloudWatch
	s3Policy, err := iam.NewRolePolicy(ctx, "lambda-s3-policy", &iam.RolePolicyArgs{
		Role: role.Name,
		Policy: pulumi.Sprintf(`{
			"Version": "2012-10-17",
			"Statement": [{
				"Effect": "Allow",
				"Action": [
					"s3:PutObject"
				],
				"Resource": "arn:aws:s3:::%s/*"
			}]
		}`, s3BucketID),
	})

	if err != nil {
		return nil, err
	}

	// Set arguments for constructing the function resource.
	args := &lambda.FunctionArgs{
		Handler: pulumi.String("handler"),
		Role:    role.Arn,
		Runtime: pulumi.String("go1.x"),
		Code:    pulumi.NewFileArchive("./build/webhook-handler.zip"),
		Environment: lambda.FunctionEnvironmentArgs{
			Variables: pulumi.StringMap{
				"BUCKET": s3BucketID,
			},
		},
	}

	// Create the lambda using the args.
	function, err := lambda.NewFunction(
		ctx,
		"answering-machine-webhook",
		args,
		pulumi.DependsOn([]pulumi.Resource{logPolicy, s3Policy}),
	)

	return function, err
}
