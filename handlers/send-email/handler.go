package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/ses"
)

type transcribeJob struct {
	JobName   string `json:"jobName"`
	AccountID string `json:"accountId"`
	Results   struct {
		Transcripts []struct {
			Transcript string `json:"transcript"`
		} `json:"transcripts"`
		Items []struct {
			StartTime    string `json:"start_time,omitempty"`
			EndTime      string `json:"end_time,omitempty"`
			Alternatives []struct {
				Confidence string `json:"confidence"`
				Content    string `json:"content"`
			} `json:"alternatives"`
			Type string `json:"type"`
		} `json:"items"`
	} `json:"results"`
	Status string `json:"status"`
}

func handler(s3Event events.S3Event) error {
	sess := session.Must(session.NewSession())
	sesClient := ses.New(sess)
	s3Client := s3.New(sess)

	log.Printf("Invoked!")

	for _, record := range s3Event.Records {
		log.Printf("[%s - %s] Bucket = %s, Key = %s \n", record.EventSource, record.EventTime, record.S3.Bucket.Name, record.S3.Object.Key)

		result, err := s3Client.GetObject(&s3.GetObjectInput{
			Bucket: aws.String(record.S3.Bucket.Name),
			Key:    aws.String(record.S3.Object.Key),
		})

		if err != nil {
			return err
		}

		defer result.Body.Close()
		body, err := ioutil.ReadAll(result.Body)
		if err != nil {
			return err
		}

		bodyString := fmt.Sprintf("%s", body)

		var job transcribeJob
		decoder := json.NewDecoder(strings.NewReader(bodyString))
		err = decoder.Decode(&job)
		if err != nil {
			return err
		}

		fmt.Println(job.JobName)
		fmt.Println(job.AccountID)
		fmt.Println(job.Results.Transcripts[0].Transcript)

		emailParams := &ses.SendEmailInput{
			Message: &ses.Message{
				Body: &ses.Body{
					Text: &ses.Content{
						Data: aws.String(job.Results.Transcripts[0].Transcript),
					},
				},
				Subject: &ses.Content{
					Data: aws.String("New Voicemail"),
				},
			},
			Destination: &ses.Destination{
				ToAddresses: []*string{aws.String(os.Getenv("TO_EMAIL"))},
			},
			Source: aws.String(os.Getenv("TO_EMAIL")),
		}

		_, err = sesClient.SendEmail(emailParams)

		if err != nil {
			return err
		}
	}

	return nil
}

func main() {
	lambda.Start(handler)
}
