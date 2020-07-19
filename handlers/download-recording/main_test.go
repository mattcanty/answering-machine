package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/aws/aws-sdk-go/service/s3/s3manager/s3manageriface"
)

type mockUploaderAPI struct {
	s3manageriface.UploaderAPI

	t            *testing.T
	expectedIn   s3manager.UploadInput
	uploadOutput s3manager.UploadOutput
	mockHits     int
}

func (mock mockUploaderAPI) Upload(in *s3manager.UploadInput, s3manager ...func(*s3manager.Uploader)) (*s3manager.UploadOutput, error) {
	// assert.Equal(mock.t, mock.expectedIn, *in, "they should be equal")

	return &mock.uploadOutput, nil
}

type mockHTTPClient struct {
	httpclientiface

	t        *testing.T
	response io.ReadCloser
	mockHits int
}

func (mock mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return &http.Response{
		Body: mock.response,
	}, nil
}

func TestLambdaHandler(t *testing.T) {
	t.Run("Successful Request", func(t *testing.T) {
		bucket := "test"
		recordingSID := "123ABC"
		recordingURL := fmt.Sprintf("https://example.com/%s", recordingSID)
		mockResponse := "Hello, World!"

		mockHTTPClient := mockHTTPClient{
			t:        t,
			response: ioutil.NopCloser(bytes.NewReader([]byte(mockResponse))),
		}

		mockUploaderAPI := mockUploaderAPI{
			t:            t,
			uploadOutput: s3manager.UploadOutput{},
			expectedIn: s3manager.UploadInput{
				Bucket: aws.String(bucket),
				Key:    aws.String(recordingSID),
				Body:   bytes.NewReader([]byte(mockResponse)),
			},
		}

		deps := deps{
			s3uploader: mockUploaderAPI,
			httpClient: mockHTTPClient,
			bucketName: bucket,
		}

		newImage := make(map[string]events.DynamoDBAttributeValue)
		newImage["RecordingSid"] = events.NewStringAttribute(recordingSID)
		newImage["RecordingUrl"] = events.NewStringAttribute(recordingURL)

		err := deps.handler(aws.BackgroundContext(), events.DynamoDBEvent{
			Records: []events.DynamoDBEventRecord{
				{
					Change: events.DynamoDBStreamRecord{
						NewImage: newImage,
					},
				},
			},
		})

		if err != nil {
			t.Error("Everything should OK")
		}
	})
}
