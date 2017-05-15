package main

import (
	"context"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func TestAlb_Register(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") == "" {
		t.Skip("Integration Test")
		return
	}

	sess, err := session.NewSession(&aws.Config{Region: aws.String("us-west-2")})
	if err != nil {
		assert.Nil(t, err)
		return
	}

	a := &alb{
		instanceId: "i-0b3c0c548063f29d2",
		LBName:     "internal",
		Suffix:     ".default.local",
		client:     elbv2.New(sess),
	}
	err = a.Open()
	assert.Nil(t, err)

	err = a.Register(
		context.Background(),
		"test-svc",
		3000,
		"/healthz",
	)
	assert.Nil(t, err)

	err = a.DeRegister(
		context.Background(),
		"test-svc",
		3000,
	)
	assert.Nil(t, err)
}
