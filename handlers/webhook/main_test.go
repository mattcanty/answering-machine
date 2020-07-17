package main

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
)

type mock struct {
	dynamodbiface.DynamoDBAPI

	t          *testing.T
	expectedIn dynamodb.PutItemInput
	mockOut    dynamodb.PutItemOutput
	tableName  string
}

func (mock mock) PutItem(in *dynamodb.PutItemInput) (*dynamodb.PutItemOutput, error) {
	assert.Equal(mock.t, mock.expectedIn, *in, "they should be equal")

	return &mock.mockOut, nil
}

func TestLambdaHandler(t *testing.T) {
	t.Run("Successful Request", func(t *testing.T) {
		tableName := "test"
		recordingSID := "123ABC"
		recordingURL := "https://example.com/recording"

		mock := mock{
			t: t,
			expectedIn: dynamodb.PutItemInput{
				Item: map[string]*dynamodb.AttributeValue{
					"RecordingSid": {
						S: aws.String(recordingSID),
					},
					"RecordingUrl": {
						S: aws.String(recordingURL),
					},
				},
				TableName: aws.String(tableName),
			},
		}

		deps := deps{
			dynamodb:  mock,
			tableName: tableName,
		}

		_, err := deps.handler(events.APIGatewayProxyRequest{
			Body: fmt.Sprintf("RecordingSid=%s&RecordingUrl=%s", recordingSID, recordingURL),
		})
		if err != nil {
			t.Error("Everything should OK")
		}
	})
}
