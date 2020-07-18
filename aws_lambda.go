package main

import (
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v2/go/aws/iam"
	"github.com/pulumi/pulumi-aws/sdk/v2/go/aws/lambda"
	"github.com/pulumi/pulumi/sdk/v2/go/pulumi"
)

func makeLambda(
	ctx *pulumi.Context,
	name string,
	statementEntries []policyStatementEntry,
	env lambda.FunctionEnvironmentArgs) (*lambda.Function, error) {

	roleName := fmt.Sprintf("answering-machine-%s-lambda-role", name)
	policyName := fmt.Sprintf("answering-machine-%s-lambda-policy", name)
	functionName := fmt.Sprintf("answering-machine-%s", name)

	assumeRolePolicy, err := newAssumeRolePolicyDocumentString("lambda.amazonaws.com")
	if err != nil {
		return &lambda.Function{}, err
	}

	role, err := iam.NewRole(ctx, roleName, &iam.RoleArgs{
		AssumeRolePolicy: pulumi.String(assumeRolePolicy),
	})
	if err != nil {
		return &lambda.Function{}, err
	}

	defaultStatements := []policyStatementEntry{
		{
			Effect: "Allow",
			Action: []string{
				"logs:CreateLogGroup",
				"logs:CreateLogStream",
				"logs:PutLogEvents",
			},
			Resource: []string{
				"arn:aws:logs:*:*:*",
			},
		},
		{
			Effect: "Allow",
			Action: []string{
				"xray:PutTraceSegments",
				"xray:PutTelemetryRecords",
				"xray:GetSamplingRules",
				"xray:GetSamplingTargets",
				"xray:GetSamplingStatisticSummaries",
			},
			Resource: []string{
				"*",
			},
		},
	}

	statements := append(statementEntries, defaultStatements...)

	policy, strArgs, err := newPolicyDocumentString(statements...)
	if err != nil {
		return &lambda.Function{}, err
	}

	rolePolicy, err := iam.NewRolePolicy(ctx, policyName, &iam.RolePolicyArgs{
		Role:   role.Name,
		Policy: pulumi.Sprintf(policy, strArgs...),
	})

	args := &lambda.FunctionArgs{
		Handler:     pulumi.String("send-email-handler"),
		Role:        role.Arn,
		Runtime:     pulumi.String("go1.x"),
		Code:        pulumi.NewFileArchive("./build/send-email-handler.zip"),
		Environment: env,
		TracingConfig: lambda.FunctionTracingConfigArgs{
			Mode: pulumi.String("Active"),
		},
	}

	function, err := lambda.NewFunction(
		ctx,
		functionName,
		args,
		pulumi.DependsOn([]pulumi.Resource{rolePolicy}),
	)

	return function, err
}
