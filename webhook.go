package main

import (
	"github.com/pulumi/pulumi-aws/sdk/v2/go/aws"
	"github.com/pulumi/pulumi-aws/sdk/v2/go/aws/apigateway"
	"github.com/pulumi/pulumi-aws/sdk/v2/go/aws/dynamodb"
	"github.com/pulumi/pulumi-aws/sdk/v2/go/aws/iam"
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

	role, err := iam.NewRole(ctx, "answering-machine-webhook-lambda-role", &iam.RoleArgs{
		AssumeRolePolicy: pulumi.String(`{
			"Version": "2012-10-17",
			"Statement": [{
				"Sid": "",
				"Effect": "Allow",
				"Principal": {
					"Service": "lambda.amazonaws.com"
				},
				"Action": "sts:AssumeRole"
			}]
		}`),
	})

	if err != nil {
		return dynamodb.Table{}, err
	}

	logPolicy, err := iam.NewRolePolicy(ctx, "answering-machine-webhook-lambda-log-policy", &iam.RolePolicyArgs{
		Role: role.Name,
		Policy: pulumi.String(`{
			"Version": "2012-10-17",
			"Statement": [{
				"Effect": "Allow",
				"Action": [
					"logs:CreateLogGroup",
					"logs:CreateLogStream",
					"logs:PutLogEvents"
				],
				"Resource": "arn:aws:logs:*:*:*"
			}]
		}`),
	})

	dynamodbPolicy, err := iam.NewRolePolicy(ctx, "answering-machine-webhook-lambda-s3-policy", &iam.RolePolicyArgs{
		Role: role.Name,
		Policy: pulumi.Sprintf(`{
			"Version": "2012-10-17",
			"Statement": [{
				"Effect": "Allow",
				"Action": [
					"dynamodb:PutItem"
				],
				"Resource": "arn:aws:dynamodb:*:*:table/%s"
			}]
		}`, dynamodbTable.ID()),
	})

	xrayPolicy, err := iam.NewRolePolicy(ctx, "answering-machine-webhook-lambda-lambda-xray-policy", &iam.RolePolicyArgs{
		Role: role.Name,
		Policy: pulumi.String(`{
			"Version": "2012-10-17",
			"Statement": [{
				"Effect": "Allow",
				"Action": [
					"xray:PutTraceSegments",
					"xray:PutTelemetryRecords",
					"xray:GetSamplingRules",
					"xray:GetSamplingTargets",
					"xray:GetSamplingStatisticSummaries"
				],
				"Resource": "*"
			}]
		}`),
	})

	if err != nil {
		return dynamodb.Table{}, err
	}

	args := &lambda.FunctionArgs{
		Handler: pulumi.String("webhook-handler"),
		Role:    role.Arn,
		Runtime: pulumi.String("go1.x"),
		Code:    pulumi.NewFileArchive("./build/webhook-handler.zip"),
		Environment: lambda.FunctionEnvironmentArgs{
			Variables: pulumi.StringMap{
				"TABLE": dynamodbTable.ID(),
			},
		},
		TracingConfig: lambda.FunctionTracingConfigArgs{
			Mode: pulumi.String("Active"),
		},
	}

	function, err := lambda.NewFunction(
		ctx,
		"answering-machine-webhook-lambda-function",
		args,
		pulumi.DependsOn([]pulumi.Resource{logPolicy, dynamodbPolicy, xrayPolicy}),
	)

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

	_, err = apigateway.NewDeployment(ctx, "answering-machine-webhook-api-deployment", &apigateway.DeploymentArgs{
		Description:      pulumi.String("Answering Machine API deployment"),
		RestApi:          gateway.ID(),
		StageDescription: pulumi.String("dev"),
		StageName:        pulumi.String("dev"),
	}, pulumi.DependsOn([]pulumi.Resource{gateway, webhookResource, function, permission}))
	if err != nil {
		return dynamodb.Table{}, err
	}

	return *dynamodbTable, nil
}
