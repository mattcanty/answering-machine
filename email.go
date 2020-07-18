package main

import (
	"os"

	"github.com/pulumi/pulumi-aws/sdk/v2/go/aws/dynamodb"
	"github.com/pulumi/pulumi-aws/sdk/v2/go/aws/lambda"
	"github.com/pulumi/pulumi-aws/sdk/v2/go/aws/s3"
	"github.com/pulumi/pulumi/sdk/v2/go/pulumi"
)

func configureSendEmail(ctx *pulumi.Context, answeringMachineTable dynamodb.Table, recordingBucketID pulumi.IDOutput, transcriptionBucketID pulumi.IDOutput) error {
	statementEntries := []policyStatementEntry{
		{
			Effect:   "Allow",
			Action:   []string{"ses:SendRawEmail"},
			Resource: []string{"*"},
		},
		{
			Effect: "Allow",
			Action: []string{"s3:GetObject"},
			Resource: []string{
				"arn:aws:s3:::%s/*",
				"arn:aws:s3:::%s/*",
			},
			resourceArgs: []interface{}{
				recordingBucketID,
				transcriptionBucketID,
			},
		},
		{
			Effect: "Allow",
			Action: []string{"dynamodb:GetItem"},
			Resource: []string{
				"%s",
			},
			resourceArgs: []interface{}{
				answeringMachineTable.Arn,
			},
		},
	}

	env := lambda.FunctionEnvironmentArgs{
		Variables: pulumi.StringMap{
			"ANSWERING_MACHINE_TABLE":                answeringMachineTable.ID(),
			"ANSWERING_MACHINE_RECORDING_BUCKET":     recordingBucketID,
			"ANSEWRING_MACHINE_TRANSCRIPTION_BUCKET": transcriptionBucketID,
			"TO_EMAIL":                               pulumi.String(os.Getenv("TO_EMAIL")),
		},
	}

	function, err := makeLambda(ctx, "send-email", statementEntries, env)
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

	return err
}
