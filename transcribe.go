package main

import (
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

	statementEntries := []policyStatementEntry{
		{
			Effect:   "Allow",
			Action:   []string{"transcribe:StartTranscriptionJob"},
			Resource: []string{"*"},
		},
		{
			Effect:       "Allow",
			Action:       []string{"s3:GetObject"},
			Resource:     []string{"arn:aws:s3:::%s/*"},
			resourceArgs: []interface{}{recordingBucketID},
		},
		{
			Effect:       "Allow",
			Action:       []string{"s3:PutObject"},
			Resource:     []string{"arn:aws:s3:::%s/*"},
			resourceArgs: []interface{}{transcribeBucket.ID()},
		},
	}

	env := lambda.FunctionEnvironmentArgs{
		Variables: pulumi.StringMap{
			"TRANSCRIPTION_BUCKET_NAME": transcribeBucket.ID(),
		},
	}

	function, err := makeLambda(ctx, "invoke-transcribe", statementEntries, env)
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
