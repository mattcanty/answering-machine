package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/textproto"
	"os"
	"strings"

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
	"github.com/aws/aws-sdk-go/service/ses"
	"github.com/aws/aws-sdk-go/service/ses/sesiface"
)

type deps struct {
	ses                   sesiface.SESAPI
	dynamodb              dynamodbiface.DynamoDBAPI
	s3                    s3manageriface.DownloadWithIterator
	toEmail               string
	answeringMachineTable string
	recordingBucket       string
	transcriptionBucket   string
}

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

type webhookData struct {
	RecordingSid string
	Caller       string
}

func (deps *deps) handler(s3Event events.S3Event) error {
	for _, record := range s3Event.Records {

		recordingSID := strings.Split(record.S3.Object.Key, ".")[0]

		log.Printf("recordingSID: %s", recordingSID)

		recordingFilePath := fmt.Sprintf("/tmp/%s.mp3", recordingSID)
		transcriptFilePath := fmt.Sprintf("/tmp/%s.json", recordingSID)

		log.Printf("recordingFilePath: %s", recordingFilePath)
		log.Printf("transcriptFilePath: %s", transcriptFilePath)

		recordingFile, err := os.Create(recordingFilePath)
		if err != nil {
			return err
		}

		log.Print("Opened recording file")

		transcriptFile, err := os.Create(transcriptFilePath)
		if err != nil {
			return err
		}

		log.Print("Opened transcript file")

		iter := &s3manager.DownloadObjectsIterator{
			Objects: []s3manager.BatchDownloadObject{
				{
					Object: &s3.GetObjectInput{
						Bucket: aws.String(deps.recordingBucket),
						Key:    aws.String(recordingSID),
					},
					Writer: recordingFile,
				},
				{
					Object: &s3.GetObjectInput{
						Bucket: aws.String(record.S3.Bucket.Name),
						Key:    aws.String(record.S3.Object.Key),
					},
					Writer: transcriptFile,
				},
			},
		}

		log.Print("Downloading files now.")

		err = deps.s3.DownloadWithIterator(aws.BackgroundContext(), iter)

		if err != nil {
			return err
		}

		log.Print("Reading transcript.")
		file, _ := ioutil.ReadFile(transcriptFilePath)
		if err != nil {
			return err
		}

		job := &transcribeJob{}
		err = json.Unmarshal([]byte(file), &job)
		if err != nil {
			return err
		}

		log.Print("Getting webhook data.")
		result, err := deps.dynamodb.GetItem(&dynamodb.GetItemInput{
			TableName: aws.String(deps.answeringMachineTable),
			Key: map[string]*dynamodb.AttributeValue{
				"RecordingSid": {
					S: aws.String(recordingSID),
				},
			},
		})
		if err != nil {
			return err
		}

		webhookData := webhookData{}
		err = dynamodbattribute.UnmarshalMap(result.Item, &webhookData)
		if err != nil {
			return err
		}

		subject := fmt.Sprintf("New voicemail from %s", webhookData.Caller)
		recording, err := ioutil.ReadFile(recordingFilePath)
		if err != nil {
			return err
		}

		input, err := buildEmailInput(
			deps.toEmail,
			deps.toEmail,
			subject,
			job.Results.Transcripts[0].Transcript,
			recording,
		)
		if err != nil {
			return err
		}

		_, err = deps.ses.SendRawEmail(input)
		if err != nil {
			return err
		}
	}

	return nil
}

func main() {
	sess := session.Must(session.NewSession())

	ses := ses.New(sess)
	dynamodb := dynamodb.New(sess)
	s3downloader := s3manager.NewDownloader(sess)

	deps := deps{
		ses:                   ses,
		dynamodb:              dynamodb,
		s3:                    s3downloader,
		toEmail:               os.Getenv("TO_EMAIL"),
		answeringMachineTable: os.Getenv("ANSWERING_MACHINE_TABLE"),
		recordingBucket:       os.Getenv("ANSWERING_MACHINE_RECORDING_BUCKET"),
		transcriptionBucket:   os.Getenv("ANSEWRING_MACHINE_TRANSCRIPTION_BUCKET"),
	}

	lambda.Start(deps.handler)
}

// https://gist.github.com/carelvwyk/60100f2421c6284391d08374bc887dca
func buildEmailInput(source, destination, subject, message string, fileContent []byte) (*ses.SendRawEmailInput, error) {

	log.Printf("source: %s", source)
	log.Printf("destination: %s", destination)
	log.Printf("subject: %s", subject)
	log.Printf("message: %s", message)

	buf := new(bytes.Buffer)
	writer := multipart.NewWriter(buf)

	// email main header:
	h := make(textproto.MIMEHeader)
	h.Set("From", source)
	h.Set("To", destination)
	h.Set("Return-Path", source)
	h.Set("Subject", subject)
	h.Set("Content-Language", "en-US")
	h.Set("Content-Type", "multipart/mixed; boundary=\""+writer.Boundary()+"\"")
	h.Set("MIME-Version", "1.0")
	_, err := writer.CreatePart(h)
	if err != nil {
		return nil, err
	}

	// body:
	h = make(textproto.MIMEHeader)
	h.Set("Content-Transfer-Encoding", "7bit")
	h.Set("Content-Type", "text/plain; charset=us-ascii")
	part, err := writer.CreatePart(h)
	if err != nil {
		return nil, err
	}
	_, err = part.Write([]byte(message))
	if err != nil {
		return nil, err
	}

	// file attachment:
	fn := "voicemail.mp3"
	h = make(textproto.MIMEHeader)
	h.Set("Content-Disposition", "attachment")
	h.Set("Content-Type", "audio/mpeg; name=\""+fn+"\"")
	h.Set("Content-Transfer-Encoding", "base64")
	part, err = writer.CreatePart(h)
	if err != nil {
		return nil, err
	}

	// Encode as base64.
	encoded := base64.StdEncoding.EncodeToString(fileContent)

	_, err = part.Write([]byte(encoded))
	if err != nil {
		return nil, err
	}
	err = writer.Close()
	if err != nil {
		return nil, err
	}

	// Strip boundary line before header (doesn't work with it present)
	s := buf.String()
	if strings.Count(s, "\n") < 2 {
		return nil, fmt.Errorf("invalid e-mail content")
	}
	s = strings.SplitN(s, "\n", 2)[1]

	raw := ses.RawMessage{
		Data: []byte(s),
	}
	input := &ses.SendRawEmailInput{
		Destinations: []*string{aws.String(destination)},
		Source:       aws.String(source),
		RawMessage:   &raw,
	}

	return input, nil
}
