package main

import (
	"os"

	"github.com/pulumi/pulumi-aws/sdk/v2/go/aws/dynamodb"
	"github.com/pulumi/pulumi-aws/sdk/v2/go/aws/lambda"
	"github.com/pulumi/pulumi/sdk/v2/go/pulumi"
)

func configureSendEmail(
	ctx *pulumi.Context,
	answeringMachineTable, transcriptionTable dynamodb.Table,
	recordingBucketID pulumi.IDOutput) error {

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
			},
			resourceArgs: []interface{}{recordingBucketID},
		},
		{
			Effect: "Allow",
			Action: []string{"dynamodb:GetItem"},
			Resource: []string{
				"%s",
				"%s",
			},
			resourceArgs: []interface{}{
				answeringMachineTable.Arn,
				transcriptionTable.Arn,
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
			resourceArgs: []interface{}{transcriptionTable.StreamArn},
		},
	}

	env := lambda.FunctionEnvironmentArgs{
		Variables: pulumi.StringMap{
			"ANSWERING_MACHINE_WEBHOOK_DATA_TABLE": answeringMachineTable.ID(),
			"ANSWERING_MACHINE_TRANSCRIPTON_TABLE": transcriptionTable.ID(),
			"ANSWERING_MACHINE_RECORDING_BUCKET":   recordingBucketID,
			"TO_EMAIL":                             pulumi.String(os.Getenv("TO_EMAIL")),
		},
	}

	function, err := makeLambda(ctx, "send-email", statementEntries, env)
	if err != nil {
		return err
	}

	_, err = lambda.NewPermission(ctx, "answering-machine-google-transcript-ready-lambda-permission", &lambda.PermissionArgs{
		Action:    pulumi.String("lambda:InvokeFunction"),
		Function:  function.Name,
		Principal: pulumi.String("s3.amazonaws.com"),
		SourceArn: transcriptionTable.StreamArn,
	})
	if err != nil {
		return err
	}

	_, err = lambda.NewEventSourceMapping(ctx, "answering-machine-google-transcript-ready", &lambda.EventSourceMappingArgs{
		EventSourceArn:   transcriptionTable.StreamArn,
		FunctionName:     function.Arn,
		StartingPosition: pulumi.String("LATEST"),
	})
	if err != nil {
		return err
	}

	return err
}
