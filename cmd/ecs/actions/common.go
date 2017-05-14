package actions

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
)

var sessions = map[string]*session.Session{}

func getSession(region string) (*session.Session, error) {
	if sess, ok := sessions[region]; ok {
		return sess, nil
	}
	sess, err := session.NewSession(&aws.Config{Region: aws.String(region)})
	if err != nil {
		return nil, err
	}
	sessions[region] = sess
	return sess, nil
}
