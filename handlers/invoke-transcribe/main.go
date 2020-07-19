package main

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/transcribeservice"
	"github.com/aws/aws-sdk-go/service/transcribeservice/transcribeserviceiface"
	"github.com/aws/aws-xray-sdk-go/xray"
)

type deps struct {
	transcribeservice       transcribeserviceiface.TranscribeServiceAPI
	transcriptionBucketName string
}

func (deps *deps) handler(ctx context.Context, s3Event events.S3Event) error {
	for _, record := range s3Event.Records {
		jobInput := &transcribeservice.StartTranscriptionJobInput{
			LanguageCode: aws.String("en-GB"),
			Media: &transcribeservice.Media{
				MediaFileUri: aws.String(fmt.Sprintf("s3://%s/%s", record.S3.Bucket.Name, record.S3.Object.Key)),
			},
			TranscriptionJobName: aws.String(record.S3.Object.Key),
			OutputBucketName:     aws.String(deps.transcriptionBucketName),
		}

		_, err := deps.transcribeservice.StartTranscriptionJobWithContext(ctx, jobInput)

		if err != nil {
			return err
		}
	}

	return nil
}

func main() {
	sess := session.Must(session.NewSession())
	transcribe := transcribeservice.New(sess)

	xray.AWS(transcribe.Client)

	deps := deps{
		transcribeservice:       transcribe,
		transcriptionBucketName: os.Getenv("TRANSCRIPTION_BUCKET_NAME"),
	}

	lambda.Start(deps.handler)
}
