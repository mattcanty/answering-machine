package main

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	speech "cloud.google.com/go/speech/apiv1p1beta1"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/aws/aws-sdk-go/service/s3/s3manager/s3manageriface"
	"github.com/aws/aws-xray-sdk-go/xray"
	"golang.org/x/net/context"
	speechpb "google.golang.org/genproto/googleapis/cloud/speech/v1p1beta1"
)

type deps struct {
	dynamodb               dynamodbiface.DynamoDBAPI
	s3                     s3manageriface.DownloaderAPI
	transcriptionTableName string
	googleAuthJSONBase64   string
}

func (deps *deps) handler(ctx context.Context, s3Event events.S3Event) error {
	authFile, err := os.Create("/tmp/auth.json")
	if err != nil {
		log.Fatal(err)
	}

	authString, err := base64.StdEncoding.DecodeString(deps.googleAuthJSONBase64)
	if err != nil {
		log.Fatal(err)
	}

	_, err = authFile.WriteString(string(authString))
	if err != nil {
		log.Fatal(err)
	}

	client, err := speech.NewClient(ctx)
	if err != nil {
		log.Fatal(err)
	}

	for _, record := range s3Event.Records {
		recordingSID := strings.Split(record.S3.Object.Key, ".")[0]
		recordingFilePath := fmt.Sprintf("/tmp/%s.mp3", recordingSID)

		recordingFile, err := os.Create(recordingFilePath)
		if err != nil {
			return err
		}

		_, err = deps.s3.DownloadWithContext(ctx, recordingFile, &s3.GetObjectInput{
			Bucket: aws.String(record.S3.Bucket.Name),
			Key:    aws.String(record.S3.Object.Key),
		})
		if err != nil {
			return err
		}

		audioData, err := ioutil.ReadFile(recordingFilePath)
		if err != nil {
			log.Fatal(err)
		}

		googleSpeechResponse, err := client.Recognize(ctx, &speechpb.RecognizeRequest{
			Config: &speechpb.RecognitionConfig{
				Encoding:        speechpb.RecognitionConfig_MP3,
				SampleRateHertz: 22000,
				LanguageCode:    "en-US",
			},
			Audio: &speechpb.RecognitionAudio{
				AudioSource: &speechpb.RecognitionAudio_Content{Content: audioData},
			},
		})
		if err != nil {
			log.Fatal(err)
		}

		params := make(map[string]string)
		params["Transcription"] = googleSpeechResponse.Results[0].Alternatives[0].Transcript
		params["RecordingSid"] = recordingSID

		attributeValues, err := dynamodbattribute.MarshalMap(params)
		if err != nil {
			return err
		}

		_, err = deps.dynamodb.PutItemWithContext(ctx, &dynamodb.PutItemInput{
			Item:      attributeValues,
			TableName: aws.String(deps.transcriptionTableName),
		})
		if err != nil {
			return err
		}
	}

	return err
}

func main() {
	sess := session.Must(session.NewSession())

	dynamodb := dynamodb.New(sess)
	s3client := s3.New(sess)

	xray.AWS(dynamodb.Client)
	xray.AWS(s3client.Client)

	s3downloader := s3manager.NewDownloaderWithClient(s3client)

	deps := deps{
		dynamodb:               dynamodb,
		s3:                     s3downloader,
		transcriptionTableName: os.Getenv("ANSWERING_MACHINE_TRANSCRIPTON_TABLE"),
		googleAuthJSONBase64:   os.Getenv("GOOGLE_AUTH_JSON_B64"),
	}

	lambda.Start(deps.handler)
}
