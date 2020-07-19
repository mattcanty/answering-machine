package main

import (
	"github.com/pulumi/pulumi-aws/sdk/v2/go/aws"
	"github.com/pulumi/pulumi-aws/sdk/v2/go/aws/apigateway"
	"github.com/pulumi/pulumi-aws/sdk/v2/go/aws/dynamodb"
	"github.com/pulumi/pulumi-aws/sdk/v2/go/aws/lambda"
	"github.com/pulumi/pulumi/sdk/v2/go/pulumi"
)

func configureWebhook(ctx *pulumi.Context, account *aws.GetCallerIdentityResult, region *aws.GetRegionResult) (dynamodb.Table, error) {
	dynamodbTable, err := dynamodb.NewTable(ctx, "answering-machine-webhook-data", &dynamodb.TableArgs{
		BillingMode: pulumi.String("PAY_PER_REQUEST"),
		HashKey:     pulumi.String("RecordingSid"),
		Attributes: dynamodb.TableAttributeArray{
			dynamodb.TableAttributeArgs{
				Name: pulumi.String("RecordingSid"),
				Type: pulumi.String("S"),
			},
		},
		StreamEnabled:  pulumi.Bool(true),
		StreamViewType: pulumi.String("NEW_IMAGE"),
	})
	if err != nil {
		return dynamodb.Table{}, err
	}

	statementEntries := []policyStatementEntry{
		{
			Effect:       "Allow",
			Action:       []string{"dynamodb:PutItem"},
			Resource:     []string{"arn:aws:dynamodb:*:*:table/%s"},
			resourceArgs: []interface{}{dynamodbTable.ID()},
		},
	}

	env := lambda.FunctionEnvironmentArgs{
		Variables: pulumi.StringMap{
			"TABLE": dynamodbTable.ID(),
		},
	}

	function, err := makeLambda(ctx, "webhook", statementEntries, env)
	if err != nil {
		return dynamodb.Table{}, err
	}

	gateway, err := apigateway.NewRestApi(ctx, "answering-machine-webhook-api", &apigateway.RestApiArgs{
		Name:        pulumi.String("answering-machine-webhook-api"),
		Description: pulumi.String("Twilio recording webhook"),
		Policy: pulumi.String(`{
			"Version": "2012-10-17",
			"Statement": [{
				"Action": "sts:AssumeRole",
				"Principal": {
					"Service": "lambda.amazonaws.com"
				},
				"Effect": "Allow",
				"Sid": ""
			},
			{
				"Action": "execute-api:Invoke",
				"Resource": "*",
				"Principal": "*",
				"Effect": "Allow",
				"Sid": ""
			}]
		}`)})
	if err != nil {
		return dynamodb.Table{}, err
	}

	webhookResource, err := apigateway.NewResource(ctx, "answering-machine-webhook-api-webhook-resource", &apigateway.ResourceArgs{
		RestApi:  gateway.ID(),
		PathPart: pulumi.String("webhook"),
		ParentId: gateway.RootResourceId,
	}, pulumi.DependsOn([]pulumi.Resource{gateway}))
	if err != nil {
		return dynamodb.Table{}, err
	}

	_, err = apigateway.NewMethod(ctx, "answering-machine-webhook-api-post-method", &apigateway.MethodArgs{
		HttpMethod:    pulumi.String("POST"),
		Authorization: pulumi.String("NONE"),
		RestApi:       gateway.ID(),
		ResourceId:    webhookResource.ID(),
	}, pulumi.DependsOn([]pulumi.Resource{gateway, webhookResource}))
	if err != nil {
		return dynamodb.Table{}, err
	}

	_, err = apigateway.NewIntegration(ctx, "answering-machine-webhook-api-lambda-integration", &apigateway.IntegrationArgs{
		HttpMethod:            pulumi.String("POST"),
		IntegrationHttpMethod: pulumi.String("POST"),
		ResourceId:            webhookResource.ID(),
		RestApi:               gateway.ID(),
		Type:                  pulumi.String("AWS_PROXY"),
		Uri:                   function.InvokeArn,
	}, pulumi.DependsOn([]pulumi.Resource{gateway, webhookResource, function}))
	if err != nil {
		return dynamodb.Table{}, err
	}

	permission, err := lambda.NewPermission(ctx, "answering-machine-webhook-api-lambda-permission", &lambda.PermissionArgs{
		Action:    pulumi.String("lambda:InvokeFunction"),
		Function:  function.Name,
		Principal: pulumi.String("apigateway.amazonaws.com"),
		SourceArn: pulumi.Sprintf("arn:aws:execute-api:%s:%s:%s/*/*/*", region.Name, account.AccountId, gateway.ID()),
	}, pulumi.DependsOn([]pulumi.Resource{gateway, webhookResource, function}))
	if err != nil {
		return dynamodb.Table{}, err
	}

	deployment, err := apigateway.NewDeployment(ctx, "answering-machine-webhook-api-deployment", &apigateway.DeploymentArgs{
		RestApi: gateway.ID(),
	}, pulumi.DependsOn([]pulumi.Resource{gateway, webhookResource, function, permission}))
	if err != nil {
		return dynamodb.Table{}, err
	}

	_, err = apigateway.NewStage(ctx, "live", &apigateway.StageArgs{
		RestApi:            gateway.ID(),
		Deployment:         deployment.ID(),
		StageName:          pulumi.String("live"),
		XrayTracingEnabled: pulumi.Bool(true),
	})
	if err != nil {
		return dynamodb.Table{}, err
	}

	ctx.Export("Webhook Endpoint", pulumi.Sprintf("https://%s.execute-api.%s.amazonaws.com/live/webhook", gateway.ID(), region.Name))

	return *dynamodbTable, nil
}
