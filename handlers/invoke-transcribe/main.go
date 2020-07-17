package main

import (
	"fmt"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/transcribeservice"
	"github.com/aws/aws-xray-sdk-go/xray"
	"github.com/uber/jaeger-client-go/crossdock/log"
)

func handler(s3Event events.S3Event) error {
	sess := session.Must(session.NewSession())
	transcribe := transcribeservice.New(sess)

	xray.AWS(transcribe.Client)

	for _, record := range s3Event.Records {
		log.Printf("[%s - %s] Bucket = %s, Key = %s \n", record.EventSource, record.EventTime, record.S3.Bucket.Name, record.S3.Object.Key)

		jobInput := &transcribeservice.StartTranscriptionJobInput{
			LanguageCode: aws.String("en-GB"),
			Media: &transcribeservice.Media{
				MediaFileUri: aws.String(fmt.Sprintf("s3://%s/%s", record.S3.Bucket.Name, record.S3.Object.Key)),
			},
			TranscriptionJobName: aws.String(record.S3.Object.Key),
			OutputBucketName:     aws.String(os.Getenv("TRANSCRIPTION_BUCKET_NAME")),
		}

		_, err := transcribe.StartTranscriptionJob(jobInput)

		if err != nil {
			return err
		}
	}

	return nil
}

func main() {
	lambda.Start(handler)
}
