package main

import (
	"github.com/pulumi/pulumi-aws/sdk/v2/go/aws/iam"
	"github.com/pulumi/pulumi-aws/sdk/v2/go/aws/lambda"
	"github.com/pulumi/pulumi-aws/sdk/v2/go/aws/s3"
	"github.com/pulumi/pulumi/sdk/v2/go/pulumi"
)

func configureTranscribe(ctx *pulumi.Context, recordingBucketID pulumi.IDOutput) (pulumi.IDOutput, error) {
	transcribeBucket, err := s3.NewBucket(ctx, "answering-machine-transcriptions", &s3.BucketArgs{})
	if err != nil {
		return pulumi.IDOutput{}, err
	}

	_, err = s3.NewBucketPublicAccessBlock(ctx, "answering-machine-transcriptions-public-access-block", &s3.BucketPublicAccessBlockArgs{
		BlockPublicAcls:   pulumi.Bool(true),
		BlockPublicPolicy: pulumi.Bool(true),
		Bucket:            transcribeBucket.ID(),
	})
	if err != nil {
		return pulumi.IDOutput{}, err
	}

	role, err := iam.NewRole(ctx, "answering-machine-transcribe-lambda-role", &iam.RoleArgs{
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
		return pulumi.IDOutput{}, err
	}

	transcribePolicy, err := iam.NewRolePolicy(ctx, "answering-machine-transcribe-lambda-start-transcription-job", &iam.RolePolicyArgs{
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

	s3Policy, err := iam.NewRolePolicy(ctx, "answering-machine-transcribe-lambda-s3-policy", &iam.RolePolicyArgs{
		Role: role.Name,
		Policy: pulumi.Sprintf(`{
			"Version": "2012-10-17",
			"Statement": [{
				"Effect": "Allow",
				"Action": [
					"s3:GetObject"
				],
				"Resource": "arn:aws:s3:::%s/*"
			},{
				"Effect": "Allow",
				"Action": [
					"s3:PutObject"
				],
				"Resource": "arn:aws:s3:::%s/*"
			}]
		}`, recordingBucketID, transcribeBucket.ID()),
	})

	xrayPolicy, err := iam.NewRolePolicy(ctx, "answering-machine-transcribe-lambda-xray-policy", &iam.RolePolicyArgs{
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
		Handler: pulumi.String("invoke-transcribe-handler"),
		Role:    role.Arn,
		Runtime: pulumi.String("go1.x"),
		Code:    pulumi.NewFileArchive("./build/invoke-transcribe-handler.zip"),
		Environment: lambda.FunctionEnvironmentArgs{
			Variables: pulumi.StringMap{
				"TRANSCRIPTION_BUCKET_NAME": transcribeBucket.ID(),
			},
		},
		TracingConfig: lambda.FunctionTracingConfigArgs{
			Mode: pulumi.String("Active"),
		},
	}

	function, err := lambda.NewFunction(
		ctx,
		"answering-machine-amazon-transcribe-invoke",
		args,
		pulumi.DependsOn([]pulumi.Resource{logPolicy, transcribePolicy, s3Policy, xrayPolicy}),
	)

	if err != nil {
		return pulumi.IDOutput{}, err
	}

	_, err = lambda.NewPermission(ctx, "answering-machine-transcribe-lambda-permission", &lambda.PermissionArgs{
		Action:    pulumi.String("lambda:InvokeFunction"),
		Function:  function.Name,
		Principal: pulumi.String("s3.amazonaws.com"),
		SourceArn: pulumi.Sprintf("arn:aws:s3:::%s", recordingBucketID),
	})
	if err != nil {
		return pulumi.IDOutput{}, err
	}

	_, err = s3.NewBucketNotification(ctx, "answering-machine-new-recording", &s3.BucketNotificationArgs{
		Bucket: recordingBucketID,
		LambdaFunctions: s3.BucketNotificationLambdaFunctionArray{
			s3.BucketNotificationLambdaFunctionArgs{
				Events: pulumi.StringArray{
					pulumi.String("s3:ObjectCreated:*"),
				},
				LambdaFunctionArn: function.Arn,
			},
		},
	})

	if err != nil {
		return pulumi.IDOutput{}, err
	}

	return transcribeBucket.ID(), err
}
