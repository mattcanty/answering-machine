package main

import (
	"github.com/pulumi/pulumi-aws/sdk/v2/go/aws/dynamodb"
	"github.com/pulumi/pulumi-aws/sdk/v2/go/aws/iam"
	"github.com/pulumi/pulumi-aws/sdk/v2/go/aws/lambda"
	"github.com/pulumi/pulumi-aws/sdk/v2/go/aws/s3"
	"github.com/pulumi/pulumi/sdk/v2/go/pulumi"
)

func configureRecordingDownload(ctx *pulumi.Context, answeringMachineTable dynamodb.Table) (pulumi.IDOutput, error) {
	bucket, err := s3.NewBucket(ctx, "answering-machine-recordings", &s3.BucketArgs{})
	if err != nil {
		return pulumi.IDOutput{}, err
	}

	_, err = s3.NewBucketPublicAccessBlock(ctx, "answering-machine-recordings-public-access-block", &s3.BucketPublicAccessBlockArgs{
		BlockPublicAcls:   pulumi.Bool(true),
		BlockPublicPolicy: pulumi.Bool(true),
		Bucket:            bucket.ID(),
	})
	if err != nil {
		return pulumi.IDOutput{}, err
	}

	role, err := iam.NewRole(ctx, "answering-machine-recording-download-lambda-role", &iam.RoleArgs{
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
		return pulumi.IDOutput{}, err
	}

	logPolicy, err := iam.NewRolePolicy(ctx, "answering-machine-recording-download-lambda-log-policy", &iam.RolePolicyArgs{
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
		return pulumi.IDOutput{}, err
	}

	transcribePolicy, err := iam.NewRolePolicy(ctx, "answering-machine-recording-download-lambda-start-transcription-job", &iam.RolePolicyArgs{
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
		return pulumi.IDOutput{}, err
	}

	s3Policy, err := iam.NewRolePolicy(ctx, "answering-machine-recording-download-lambda-s3-policy", &iam.RolePolicyArgs{
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
		}`, bucket.ID()),
	})

	streamPolicy, err := iam.NewRolePolicy(ctx, "answering-machine-recording-download-stream-policy", &iam.RolePolicyArgs{
		Role: role.Name,
		Policy: pulumi.Sprintf(`{
			"Version": "2012-10-17",
			"Statement": [{
				"Effect": "Allow",
				"Action": [
					"dynamodb:GetRecords",
					"dynamodb:GetShardIterator",
					"dynamodb:DescribeStream",
					"dynamodb:ListStreams"
				],
				"Resource": "%s"
			}]
		}`, answeringMachineTable.StreamArn),
	})

	xrayPolicy, err := iam.NewRolePolicy(ctx, "answering-machine-recording-download-lambda-xray-policy", &iam.RolePolicyArgs{
		Role: role.Name,
		Policy: pulumi.String(`{
			"Version": "2012-10-17",
			"Statement": [{
				"Effect": "Allow",
				"Action": [
					"xray:PutTraceSegments",
					"xray:PutTelemetryRecords",
					"xray:GetSamplingRules",
					"xray:GetSamplingTargets",
					"xray:GetSamplingStatisticSummaries"
				],
				"Resource": "*"
			}]
		}`),
	})

	args := &lambda.FunctionArgs{
		Handler: pulumi.String("download-recording-handler"),
		Role:    role.Arn,
		Runtime: pulumi.String("go1.x"),
		Code:    pulumi.NewFileArchive("./build/download-recording-handler.zip"),
		Environment: lambda.FunctionEnvironmentArgs{
			Variables: pulumi.StringMap{
				"RECORDING_BUCKET_NAME": bucket.ID(),
			},
		},
		TracingConfig: lambda.FunctionTracingConfigArgs{
			Mode: pulumi.String("Active"),
		},
	}

	function, err := lambda.NewFunction(
		ctx,
		"answering-machine-recording-download",
		args,
		pulumi.DependsOn([]pulumi.Resource{logPolicy, transcribePolicy, s3Policy, xrayPolicy, streamPolicy}),
	)

	if err != nil {
		return pulumi.IDOutput{}, err
	}

	_, err = lambda.NewPermission(ctx, "answering-machine-recording-download-lambda-permission", &lambda.PermissionArgs{
		Action:    pulumi.String("lambda:InvokeFunction"),
		Function:  function.Name,
		Principal: pulumi.String("s3.amazonaws.com"),
		SourceArn: answeringMachineTable.StreamArn,
	})
	if err != nil {
		return pulumi.IDOutput{}, err
	}

	_, err = lambda.NewEventSourceMapping(ctx, "answering-machine-new-recording", &lambda.EventSourceMappingArgs{
		EventSourceArn:   answeringMachineTable.StreamArn,
		FunctionName:     function.Arn,
		StartingPosition: pulumi.String("LATEST"),
	})

	if err != nil {
		return pulumi.IDOutput{}, err
	}

	return bucket.ID(), err
}
