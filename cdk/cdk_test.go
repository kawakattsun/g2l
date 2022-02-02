package main

import (
	"encoding/json"
	"testing"

	"github.com/aws/aws-cdk-go/awscdk"
	"github.com/stretchr/testify/assert"
	"github.com/tidwall/gjson"
)

func TestCdkStack(t *testing.T) {
	// GIVEN
	app := awscdk.NewApp(nil)

	// WHEN
	stack := NewCdkStack(app, "MyStack", nil)

	// THEN
	bytes, err := json.Marshal(app.Synth(nil).GetStackArtifact(stack.ArtifactId()).Template())
	if err != nil {
		t.Error(err)
	}

	template := gjson.ParseBytes(bytes)
	scheduleEventProperty := template.Get("Resources.G2LFunction.Properties.Events.G2LFunctionSchedule.Properties").String()
	assert.Equal(t, "MyCoolTopic", scheduleEventProperty)
}
