package main

import (
	"github.com/pulumi/pulumi-aws/sdk/v2/go/aws/iam"
	"github.com/pulumi/pulumi-aws/sdk/v2/go/aws/lambda"
	"github.com/pulumi/pulumi/sdk/v2/go/pulumi"
)

func makeTranscribeLambda(ctx *pulumi.Context, s3BucketID pulumi.IDOutput) (*lambda.Function, error) {
	// Create an IAM role.
	role, err := iam.NewRole(ctx, "answering-machine-transcribe-exec-role", &iam.RoleArgs{
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
	logPolicy, err := iam.NewRolePolicy(ctx, "answering-machine-transcribe-lambda-log-policy", &iam.RolePolicyArgs{
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

	if err != nil {
		return nil, err
	}

	transcribePolicy, err := iam.NewRolePolicy(ctx, "answering-machine-transcribe-start-transcription-job", &iam.RolePolicyArgs{
		Role: role.Name,
		Policy: pulumi.String(`{
			"Version": "2012-10-17",
			"Statement": [{
				"Effect": "Allow",
				"Action": [
					"transcribe:StartTranscriptionJob"
				],
				"Resource": "*"
			}]
		}`),
	})

	if err != nil {
		return nil, err
	}

	// Attach a policy to allow writing logs to CloudWatch
	s3Policy, err := iam.NewRolePolicy(ctx, "answering-machine-lambda-transcribe-s3-policy", &iam.RolePolicyArgs{
		Role: role.Name,
		Policy: pulumi.Sprintf(`{
			"Version": "2012-10-17",
			"Statement": [{
				"Effect": "Allow",
				"Action": [
					"s3:GetObject",
					"s3:PutObject"
				],
				"Resource": "arn:aws:s3:::%s/*"
			}]
		}`, s3BucketID),
	})

	// Set arguments for constructing the function resource.
	args := &lambda.FunctionArgs{
		Handler: pulumi.String("handler"),
		Role:    role.Arn,
		Runtime: pulumi.String("go1.x"),
		Code:    pulumi.NewFileArchive("./build/invoke-transcribe-handler.zip"),
		Environment: lambda.FunctionEnvironmentArgs{
			Variables: pulumi.StringMap{
				"BUCKET": s3BucketID,
			},
		},
	}

	// Create the lambda using the args.
	function, err := lambda.NewFunction(
		ctx,
		"answering-machine-invoke-transcribe",
		args,
		pulumi.DependsOn([]pulumi.Resource{logPolicy, transcribePolicy, s3Policy}),
	)

	return function, err
}
