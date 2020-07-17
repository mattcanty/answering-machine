package main

import (
	"github.com/pulumi/pulumi-aws/sdk/v2/go/aws"
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

		answeringMachineTable, err := configureWebhook(ctx, account, region)
		if err != nil {
			return err
		}

		recordingBucketID, err := configureRecordingDownload(ctx, answeringMachineTable)
		if err != nil {
			return err
		}

		transcriptionBucketID, err := configureTranscribe(ctx, recordingBucketID)
		if err != nil {
			return err
		}

		return configureSendEmail(ctx, answeringMachineTable, recordingBucketID, transcriptionBucketID)
	})
}
