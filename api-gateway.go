package main

import (
	"github.com/pulumi/pulumi-aws/sdk/v2/go/aws"
	"github.com/pulumi/pulumi-aws/sdk/v2/go/aws/apigateway"
	"github.com/pulumi/pulumi-aws/sdk/v2/go/aws/lambda"
	"github.com/pulumi/pulumi/sdk/v2/go/pulumi"
)

func makeAPIGateway(ctx *pulumi.Context, function *lambda.Function, account *aws.GetCallerIdentityResult, region *aws.GetRegionResult) error {
	// Create a new API Gateway.
	gateway, err := apigateway.NewRestApi(ctx, "TwilioRecordingWebhook", &apigateway.RestApiArgs{
		Name:        pulumi.String("TwilioRecordingWebhook"),
		Description: pulumi.String("An API Gateway for the Twilio recording webhook"),
		Policy: pulumi.String(`{
"Version": "2012-10-17",
"Statement": [
{
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
}
]
}`)})
	if err != nil {
		return err
	}

	// Add a resource to the API Gateway.
	// This makes the API Gateway accept requests on "/{message}".
	apiresource, err := apigateway.NewResource(ctx, "UpperAPI", &apigateway.ResourceArgs{
		RestApi:  gateway.ID(),
		PathPart: pulumi.String("webhook"),
		ParentId: gateway.RootResourceId,
	}, pulumi.DependsOn([]pulumi.Resource{gateway}))
	if err != nil {
		return err
	}

	// Add a method to the API Gateway.
	_, err = apigateway.NewMethod(ctx, "PostMethod", &apigateway.MethodArgs{
		HttpMethod:    pulumi.String("POST"),
		Authorization: pulumi.String("NONE"),
		RestApi:       gateway.ID(),
		ResourceId:    apiresource.ID(),
	}, pulumi.DependsOn([]pulumi.Resource{gateway, apiresource}))
	if err != nil {
		return err
	}

	// Add an integration to the API Gateway.
	// This makes communication between the API Gateway and the Lambda function work
	_, err = apigateway.NewIntegration(ctx, "LambdaIntegration", &apigateway.IntegrationArgs{
		HttpMethod:            pulumi.String("POST"),
		IntegrationHttpMethod: pulumi.String("POST"),
		ResourceId:            apiresource.ID(),
		RestApi:               gateway.ID(),
		Type:                  pulumi.String("AWS_PROXY"),
		Uri:                   function.InvokeArn,
	}, pulumi.DependsOn([]pulumi.Resource{gateway, apiresource, function}))
	if err != nil {
		return err
	}

	// Add a resource based policy to the Lambda function.
	// This is the final step and allows AWS API Gateway to communicate with the AWS Lambda function
	permission, err := lambda.NewPermission(ctx, "APIPermission", &lambda.PermissionArgs{
		Action:    pulumi.String("lambda:InvokeFunction"),
		Function:  function.Name,
		Principal: pulumi.String("apigateway.amazonaws.com"),
		SourceArn: pulumi.Sprintf("arn:aws:execute-api:%s:%s:%s/*/*/*", region.Name, account.AccountId, gateway.ID()),
	}, pulumi.DependsOn([]pulumi.Resource{gateway, apiresource, function}))
	if err != nil {
		return err
	}

	// Create a new deployment
	_, err = apigateway.NewDeployment(ctx, "APIDeployment", &apigateway.DeploymentArgs{
		Description:      pulumi.String("UpperCase API deployment"),
		RestApi:          gateway.ID(),
		StageDescription: pulumi.String("Production"),
		StageName:        pulumi.String("prod"),
	}, pulumi.DependsOn([]pulumi.Resource{gateway, apiresource, function, permission}))
	if err != nil {
		return err
	}

	ctx.Export("invocation URL", pulumi.Sprintf("https://%s.execute-api.%s.amazonaws.com/prod/{message}", gateway.ID(), region.Name))

	return nil
}
