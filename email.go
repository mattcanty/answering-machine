package main

import (
	"os"

	"github.com/pulumi/pulumi-aws/sdk/v2/go/aws/dynamodb"
	"github.com/pulumi/pulumi-aws/sdk/v2/go/aws/iam"
	"github.com/pulumi/pulumi-aws/sdk/v2/go/aws/lambda"
	"github.com/pulumi/pulumi-aws/sdk/v2/go/aws/s3"
	"github.com/pulumi/pulumi/sdk/v2/go/pulumi"
)

func configureSendEmail(ctx *pulumi.Context, answeringMachineTable dynamodb.Table, recordingBucketID pulumi.IDOutput, transcriptionBucketID pulumi.IDOutput) error {
	role, err := iam.NewRole(ctx, "answering-machine-send-email-lambda-role", &iam.RoleArgs{
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
		return err
	}

	logPolicy, err := iam.NewRolePolicy(ctx, "answering-machine-send-email-lambda-log-policy", &iam.RolePolicyArgs{
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
		return err
	}

	sesPolicy, err := iam.NewRolePolicy(ctx, "answering-machine-send-email-ses-policy", &iam.RolePolicyArgs{
		Role: role.Name,
		Policy: pulumi.String(`{
			"Version": "2012-10-17",
			"Statement": [{
				"Effect": "Allow",
				"Action": [
					"ses:SendRawEmail"
				],
				"Resource": "*"
			}]
		}`),
	})

	if err != nil {
		return err
	}

	s3Policy, err := iam.NewRolePolicy(ctx, "answering-machine-send-email-lambda-s3-policy", &iam.RolePolicyArgs{
		Role: role.Name,
		Policy: pulumi.Sprintf(`{
			"Version": "2012-10-17",
			"Statement": [{
				"Effect": "Allow",
				"Action": [
					"s3:GetObject"
				],
				"Resource": [
					"arn:aws:s3:::%s/*",
					"arn:aws:s3:::%s/*"
				]
			},{
				"Effect": "Allow",
				"Action": [
					"dynamodb:GetItem"
				],
				"Resource": [
					"%s"
				]
			}]
		}`, recordingBucketID, transcriptionBucketID, answeringMachineTable.Arn),
	})

	ddbPolicy, err := iam.NewRolePolicy(ctx, "answering-machine-send-email-lambda-dynamodb-policy", &iam.RolePolicyArgs{
		Role: role.Name,
		Policy: pulumi.Sprintf(`{
			"Version": "2012-10-17",
			"Statement": [{
				"Effect": "Allow",
				"Action": [
					"dynamodb:GetItem"
				],
				"Resource": [
					"%s"
				]
			}]
		}`, answeringMachineTable.Arn),
	})

	xrayPolicy, err := iam.NewRolePolicy(ctx, "answering-machine-send-email-lambda-xray-policy", &iam.RolePolicyArgs{
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
		Handler: pulumi.String("send-email-handler"),
		Role:    role.Arn,
		Runtime: pulumi.String("go1.x"),
		Code:    pulumi.NewFileArchive("./build/send-email-handler.zip"),
		Environment: lambda.FunctionEnvironmentArgs{
			Variables: pulumi.StringMap{
				"ANSWERING_MACHINE_TABLE":                answeringMachineTable.ID(),
				"ANSWERING_MACHINE_RECORDING_BUCKET":     recordingBucketID,
				"ANSEWRING_MACHINE_TRANSCRIPTION_BUCKET": transcriptionBucketID,
				"TO_EMAIL":                               pulumi.String(os.Getenv("TO_EMAIL")),
			},
		},
		TracingConfig: lambda.FunctionTracingConfigArgs{
			Mode: pulumi.String("Active"),
		},
	}

	function, err := lambda.NewFunction(
		ctx,
		"answering-machine-amazon-send-email-invoke",
		args,
		pulumi.DependsOn([]pulumi.Resource{logPolicy, sesPolicy, s3Policy, xrayPolicy, ddbPolicy}),
	)

	if err != nil {
		return err
	}

	_, err = lambda.NewPermission(ctx, "answering-machine-send-email-lambda-permission", &lambda.PermissionArgs{
		Action:    pulumi.String("lambda:InvokeFunction"),
		Function:  function.Name,
		Principal: pulumi.String("s3.amazonaws.com"),
		SourceArn: pulumi.Sprintf("arn:aws:s3:::%s", transcriptionBucketID),
	})
	if err != nil {
		return err
	}

	_, err = s3.NewBucketNotification(ctx, "answering-machine-new-transcription", &s3.BucketNotificationArgs{
		Bucket: transcriptionBucketID,
		LambdaFunctions: s3.BucketNotificationLambdaFunctionArray{
			s3.BucketNotificationLambdaFunctionArgs{
				Events: pulumi.StringArray{
					pulumi.String("s3:ObjectCreated:*"),
				},
				FilterSuffix:      pulumi.String("json"),
				LambdaFunctionArn: function.Arn,
			},
		},
	})

	if err != nil {
		return err
	}

	return err
}
