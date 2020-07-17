package main

import (
	"log"
	"net/url"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
	"github.com/aws/aws-xray-sdk-go/xray"
)

type deps struct {
	dynamodb  dynamodbiface.DynamoDBAPI
	tableName string
}

func (deps *deps) handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	params, err := url.ParseQuery(request.Body)
	if err != nil {
		log.Fatal(err)

		return events.APIGatewayProxyResponse{
			StatusCode: 400,
		}, err
	}

	fixedParams := make(map[string]string)
	for k, v := range params {
		fixedParams[k] = v[0]
	}

	attributeValues, err := dynamodbattribute.MarshalMap(fixedParams)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 400,
		}, err
	}

	_, err = deps.dynamodb.PutItem(&dynamodb.PutItemInput{
		Item:      attributeValues,
		TableName: aws.String(deps.tableName),
	})
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 400,
		}, err
	}

	return events.APIGatewayProxyResponse{
		StatusCode: 200,
	}, nil
}

func main() {
	sess := session.Must(session.NewSession())
	dynamodb := dynamodb.New(sess)

	xray.AWS(dynamodb.Client)

	deps := deps{
		dynamodb:  dynamodb,
		tableName: os.Getenv("TABLE"),
	}

	lambda.Start(deps.handler)
}
