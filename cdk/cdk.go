package main

import (
	"os"

	"github.com/aws/aws-cdk-go/awscdk"
	"github.com/aws/aws-cdk-go/awscdk/awsevents"
	"github.com/aws/aws-cdk-go/awscdk/awseventstargets"
	"github.com/aws/aws-cdk-go/awscdk/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/awslambdago"
	"github.com/aws/aws-cdk-go/awscdk/awslogs"
	"github.com/aws/constructs-go/constructs/v3"
	"github.com/aws/jsii-runtime-go"
)

type CdkStackProps struct {
	awscdk.StackProps
}

func NewCdkStack(scope constructs.Construct, id string, props *CdkStackProps) awscdk.Stack {
	var sprops awscdk.StackProps
	if props != nil {
		sprops = props.StackProps
	}
	stack := awscdk.NewStack(scope, &id, &sprops)

	// newSSMFunction(stack)
	newFunction(stack)

	return stack
}

func newFunction(stack awscdk.Construct) {
	env := map[string]*string{
		"GOOGLE_TOKEN":              jsii.String(os.Getenv("GOOGLE_TOKEN")),
		"GOOGLE_CREDENTIALS":        jsii.String(os.Getenv("GOOGLE_CREDENTIALS")),
		"LINE_CHANNEL_SECRET":       jsii.String(os.Getenv("LINE_CHANNEL_SECRET")),
		"LINE_CHANNEL_ACCESS_TOKEN": jsii.String(os.Getenv("LINE_CHANNEL_ACCESS_TOKEN")),
		"FORWARD_LINE_ID":           jsii.String(os.Getenv("FORWARD_LINE_ID")),
		"INTERVAL_MINUTES":          jsii.String(os.Getenv("INTERVAL_MINUTES")),
	}
	fn := awslambdago.NewGoFunction(stack, jsii.String("G2LFunction"), &awslambdago.GoFunctionProps{
		Runtime:      awslambda.Runtime_GO_1_X(),
		MemorySize:   jsii.Number(128),
		Environment:  &env,
		Entry:        jsii.String("../cmd/g2l"),
		ModuleDir:    jsii.String("../"),
		LogRetention: awslogs.RetentionDays_ONE_WEEK,
	})

	targets := []awsevents.IRuleTarget{
		awseventstargets.NewLambdaFunction(fn, nil),
	}
	awsevents.NewRule(stack, jsii.String("G2lScheduleEvent"), &awsevents.RuleProps{
		Schedule: awsevents.Schedule_Rate(awscdk.Duration_Minutes(jsii.Number(1))),
		Targets:  &targets,
		Enabled:  jsii.Bool(false),
	})
}

func main() {
	app := awscdk.NewApp(nil)

	NewCdkStack(app, "G2LCdkStack", &CdkStackProps{
		awscdk.StackProps{
			Env: env(),
		},
	})

	app.Synth(nil)
}

// env determines the AWS environment (account+region) in which our stack is to
// be deployed. For more information see: https://docs.aws.amazon.com/cdk/latest/guide/environments.html
func env() *awscdk.Environment {
	// If unspecified, this stack will be "environment-agnostic".
	// Account/Region-dependent features and context lookups will not work, but a
	// single synthesized template can be deployed anywhere.
	//---------------------------------------------------------------------------
	// return nil

	// Uncomment if you know exactly what account and region you want to deploy
	// the stack to. This is the recommendation for production stacks.
	//---------------------------------------------------------------------------
	// return &awscdk.Environment{
	//  Account: jsii.String("123456789012"),
	//  Region:  jsii.String("us-east-1"),
	// }

	// Uncomment to specialize this stack for the AWS Account and Region that are
	// implied by the current CLI configuration. This is recommended for dev
	// stacks.
	//---------------------------------------------------------------------------
	return &awscdk.Environment{
		Account: jsii.String(os.Getenv("CDK_DEFAULT_ACCOUNT")),
		Region:  jsii.String(os.Getenv("CDK_DEFAULT_REGION")),
	}
}
