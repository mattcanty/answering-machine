package main

import (
	"net/http"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/aws/aws-sdk-go/service/s3/s3manager/s3manageriface"
)

type deps struct {
	httpClient httpclientiface
	s3uploader s3manageriface.UploaderAPI
	bucketName string
}

type httpclientiface interface {
	Do(req *http.Request) (*http.Response, error)
}

func (deps *deps) handler(event events.DynamoDBEvent) error {
	for _, record := range event.Records {
		sid := record.Change.NewImage["RecordingSid"].String()
		url := record.Change.NewImage["RecordingUrl"].String() + ".mp3"

		request, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return err
		}

		resp, err := deps.httpClient.Do(request)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		_, err = deps.s3uploader.Upload(&s3manager.UploadInput{
			Bucket: aws.String(deps.bucketName),
			Key:    aws.String(sid),
			Body:   resp.Body,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func main() {
	sess := session.Must(session.NewSession())
	s3manager := s3manager.NewUploader(sess)

	deps := deps{
		s3uploader: s3manager,
		httpClient: &http.Client{},
		bucketName: os.Getenv("RECORDING_BUCKET_NAME"),
	}

	lambda.Start(deps.handler)
}
