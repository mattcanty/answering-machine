package main

import (
	"github.com/pulumi/pulumi-aws/sdk/v2/go/aws/dynamodb"
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

	statementEntries := []policyStatementEntry{
		{
			Effect: "Allow",
			Action: []string{"s3:PutObject"},
			Resource: []string{
				"arn:aws:s3:::%s/*",
			},
			resourceArgs: []interface{}{
				bucket.ID(),
			},
		},
		{
			Effect: "Allow",
			Action: []string{
				"dynamodb:GetRecords",
				"dynamodb:GetShardIterator",
				"dynamodb:DescribeStream",
				"dynamodb:ListStreams",
			},
			Resource:     []string{"%s"},
			resourceArgs: []interface{}{answeringMachineTable.StreamArn},
		},
	}

	env := lambda.FunctionEnvironmentArgs{
		Variables: pulumi.StringMap{
			"RECORDING_BUCKET_NAME": bucket.ID(),
		},
	}

	function, err := makeLambda(ctx, "download-recording", statementEntries, env)
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
