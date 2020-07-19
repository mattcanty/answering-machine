package main

import (
	"os"

	"github.com/pulumi/pulumi-aws/sdk/v2/go/aws/dynamodb"
	"github.com/pulumi/pulumi-aws/sdk/v2/go/aws/lambda"
	"github.com/pulumi/pulumi-aws/sdk/v2/go/aws/s3"
	"github.com/pulumi/pulumi/sdk/v2/go/pulumi"
)

func configureGoogleSpeech(ctx *pulumi.Context, recordingBucketID pulumi.IDOutput) (dynamodb.Table, error) {
	dynamodbTable, err := dynamodb.NewTable(ctx, "answering-machine-transcript-data", &dynamodb.TableArgs{
		BillingMode: pulumi.String("PAY_PER_REQUEST"),
		HashKey:     pulumi.String("RecordingSid"),
		Attributes: dynamodb.TableAttributeArray{
			dynamodb.TableAttributeArgs{
				Name: pulumi.String("RecordingSid"),
				Type: pulumi.String("S"),
			},
		},
		StreamEnabled:  pulumi.Bool(true),
		StreamViewType: pulumi.String("NEW_IMAGE"),
	})
	if err != nil {
		return dynamodb.Table{}, err
	}

	statementEntries := []policyStatementEntry{
		{
			Effect:   "Allow",
			Action:   []string{"ses:SendRawEmail"},
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
			Action:       []string{"dynamodb:PutItem"},
			Resource:     []string{"arn:aws:dynamodb:*:*:table/%s"},
			resourceArgs: []interface{}{dynamodbTable.ID()},
		},
	}

	env := lambda.FunctionEnvironmentArgs{
		Variables: pulumi.StringMap{
			"ANSWERING_MACHINE_TRANSCRIPTON_TABLE": dynamodbTable.ID(),
			"GOOGLE_AUTH_JSON_B64":                 pulumi.String(os.Getenv("GOOGLE_AUTH_JSON_B64")),
			"GOOGLE_APPLICATION_CREDENTIALS":       pulumi.String("/tmp/auth.json"),
		},
	}

	function, err := makeLambda(ctx, "google-speech", statementEntries, env)
	if err != nil {
		return dynamodb.Table{}, err
	}

	_, err = lambda.NewPermission(ctx, "answering-machine-google-speech-permission", &lambda.PermissionArgs{
		Action:    pulumi.String("lambda:InvokeFunction"),
		Function:  function.Name,
		Principal: pulumi.String("s3.amazonaws.com"),
		SourceArn: pulumi.Sprintf("arn:aws:s3:::%s", recordingBucketID),
	})
	if err != nil {
		return dynamodb.Table{}, err
	}

	_, err = s3.NewBucketNotification(ctx, "answering-machine-new-recording-google-speech", &s3.BucketNotificationArgs{
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

	return *dynamodbTable, err
}
