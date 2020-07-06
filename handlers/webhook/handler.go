package main

import (
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

// handler is a simple function that takes a string and does a ToUpper.
func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	sess := session.Must(session.NewSession())
	uploader := s3manager.NewUploader(sess)

	params, err := url.ParseQuery(request.Body)
	if err != nil {
		log.Fatal(err)

		return events.APIGatewayProxyResponse{
			StatusCode: 200,
		}, nil
	}

	recordingID := params["RecordingSid"][0]
	recordingURL := params["RecordingUrl"][0] + ".mp3"
	recordingS3Key := recordingID + ".mp3"
	recordingTempPath := path.Join("/", "tmp", recordingID)

	log.Printf("Downloading '%s'", recordingURL)

	err = downloadFile(recordingTempPath, recordingURL)
	if err != nil {
		log.Fatal(err)

		return events.APIGatewayProxyResponse{
			StatusCode: 200,
		}, nil
	}

	f, err := os.Open(recordingTempPath)
	if err != nil {
		log.Fatal(err)
		return events.APIGatewayProxyResponse{
			StatusCode: 200,
		}, nil
	}

	result, err := uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(os.Getenv("BUCKET")),
		Key:    aws.String(recordingS3Key),
		Body:   f,
	})

	if err != nil {
		log.Fatal(err)
		return events.APIGatewayProxyResponse{
			StatusCode: 200,
		}, nil
	}
	log.Printf("file uploaded to %s\n", result.Location)

	for key, value := range params {
		log.Printf("  %v = %v\n", key, value)
	}

	return events.APIGatewayProxyResponse{
		StatusCode: 200,
	}, nil
}

func main() {
	lambda.Start(handler)
}

func downloadFile(filepath string, url string) error {
	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return err
}
