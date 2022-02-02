package main

import (
	"github.com/aws/aws-cdk-go/awscdk"
	"github.com/aws/aws-cdk-go/awscdk/awsevents"
	"github.com/aws/aws-cdk-go/awscdk/awseventstargets"
	"github.com/aws/aws-cdk-go/awscdk/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/awslambdago"
	"github.com/aws/aws-cdk-go/awscdk/awslogs"
	"github.com/aws/aws-cdk-go/awscdk/awssam"
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

func newSSMFunction(stack awscdk.Construct) {
	awssam.NewCfnFunction(stack, jsii.String("G2LFunction"), &awssam.CfnFunctionProps{
		FunctionName: jsii.String("G2LFunction"),
		CodeUri:      jsii.String("../deploy/lambda/g2l"),
		Handler:      jsii.String("main"),
		Runtime:      awslambda.Runtime_GO_1_X().Name(),
		MemorySize:   jsii.Number(128),
		// Environment:  map[string]string{},
		Events: map[string]*awssam.CfnFunction_EventSourceProperty{
			"G2LFunctionSchedule": {
				Properties: awssam.CfnFunction_ScheduleEventProperty{
					Schedule: awsevents.Schedule_Rate(awscdk.Duration_Minutes(jsii.Number(1))).ExpressionString(),
				},
				Type: jsii.String("Schedule"),
			},
		},
	})
}

func newFunction(stack awscdk.Construct) {
	env := map[string]*string{}
	fn := awslambdago.NewGoFunction(stack, jsii.String("G2LFunction"), &awslambdago.GoFunctionProps{
		Runtime:      awslambda.Runtime_GO_1_X(),
		MemorySize:   jsii.Number(128),
		Environment:  &env,
		Entry:        jsii.String("../cmd/g2l"),
		ModuleDir:    jsii.String("../"),
		LogRetention: awslogs.RetentionDays_ONE_WEEK,
	})
	// fn := awslambda.NewFunction(stack, jsii.String("G2LFunction"), &awslambda.FunctionProps{
	// 	FunctionName: jsii.String("G2LFunction"),
	// 	Code:         awslambda.NewAssetCode(jsii.String("../deploy/lambda/g2l"), nil),
	// 	Handler:      jsii.String("main"),
	// 	Runtime:      awslambda.Runtime_GO_1_X(),
	// 	MemorySize:   jsii.Number(128),
	// 	Environment:  &env,
	// })

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
	return nil

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
	// return &awscdk.Environment{
	//  Account: jsii.String(os.Getenv("CDK_DEFAULT_ACCOUNT")),
	//  Region:  jsii.String(os.Getenv("CDK_DEFAULT_REGION")),
	// }
}
