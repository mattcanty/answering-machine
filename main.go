package main

import (
	"github.com/pulumi/pulumi-aws/sdk/v2/go/aws"
	"github.com/pulumi/pulumi-aws/sdk/v2/go/aws/lambda"
	"github.com/pulumi/pulumi-aws/sdk/v2/go/aws/s3"
	"github.com/pulumi/pulumi/sdk/v2/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		account, err := aws.GetCallerIdentity(ctx)
		if err != nil {
			return err
		}

		region, err := aws.GetRegion(ctx, &aws.GetRegionArgs{})
		if err != nil {
			return err
		}

		s3Bucket, err := s3.NewBucket(ctx, "twilio-answering-machine-recordings", &s3.BucketArgs{})
		if err != nil {
			return err
		}

		transcribeFunction, err := makeTranscribeLambda(ctx, s3Bucket.ID())
		if err != nil {
			return err
		}

		sendEmailFunction, err := makeSendEmailLambda(ctx, s3Bucket.ID())
		if err != nil {
			return err
		}

		_, err = lambda.NewPermission(ctx, "S3TranscribePermission", &lambda.PermissionArgs{
			Action:    pulumi.String("lambda:InvokeFunction"),
			Function:  transcribeFunction.Name,
			Principal: pulumi.String("s3.amazonaws.com"),
			SourceArn: pulumi.Sprintf("arn:aws:s3:::%s", s3Bucket.ID()),
		})
		if err != nil {
			return err
		}
		_, err = lambda.NewPermission(ctx, "S3SendEmailPermission", &lambda.PermissionArgs{
			Action:    pulumi.String("lambda:InvokeFunction"),
			Function:  sendEmailFunction.Name,
			Principal: pulumi.String("s3.amazonaws.com"),
			SourceArn: pulumi.Sprintf("arn:aws:s3:::%s", s3Bucket.ID()),
		})
		if err != nil {
			return err
		}

		_, err = s3.NewBucketNotification(ctx, "answering-machine-new-recording", &s3.BucketNotificationArgs{
			Bucket: s3Bucket.ID(),
			LambdaFunctions: s3.BucketNotificationLambdaFunctionArray{
				s3.BucketNotificationLambdaFunctionArgs{
					Events: pulumi.StringArray{
						pulumi.String("s3:ObjectCreated:*"),
					},
					FilterSuffix:      pulumi.String(".mp3"),
					LambdaFunctionArn: transcribeFunction.Arn,
				},
				s3.BucketNotificationLambdaFunctionArgs{
					Events: pulumi.StringArray{
						pulumi.String("s3:ObjectCreated:*"),
					},
					FilterSuffix:      pulumi.String(".json"),
					LambdaFunctionArn: sendEmailFunction.Arn,
				},
			},
		})

		if err != nil {
			return err
		}

		webhookFunction, err := makeWebhookLambda(ctx, s3Bucket.ID())
		if err != nil {
			return err
		}

		err = makeAPIGateway(ctx, webhookFunction, account, region)
		if err != nil {
			return err
		}

		return nil
	})
}
