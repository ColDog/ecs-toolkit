package kv

import (
	"context"
	"encoding/json"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
)

const table = "sked_objects"

func NewDynamoDB(sess *session.Session) (DB, error) {
	db := &DynamoDB{
		Client: dynamodb.New(sess),
	}
	err := db.Open()
	return db, err
}

type DynamoDB struct {
	Client *dynamodb.DynamoDB
}

func (db *DynamoDB) Open() (err error) {
	_, err = db.Client.DescribeTable(&dynamodb.DescribeTableInput{
		TableName: aws.String(table),
	})
	if err == nil {
		return nil
	}

	_, err = db.Client.CreateTable(&dynamodb.CreateTableInput{
		TableName: aws.String(table),
		KeySchema: []*dynamodb.KeySchemaElement{
			{AttributeName: aws.String("class"), KeyType: aws.String("HASH")},
			{AttributeName: aws.String("key"), KeyType: aws.String("RANGE")},
		},
		AttributeDefinitions: []*dynamodb.AttributeDefinition{
			{AttributeName: aws.String("class"), AttributeType: aws.String("S")},
			{AttributeName: aws.String("key"), AttributeType: aws.String("S")},
		},
		ProvisionedThroughput: &dynamodb.ProvisionedThroughput{
			WriteCapacityUnits: aws.Int64(1),
			ReadCapacityUnits:  aws.Int64(1),
		},
	})
	return err
}

func (db *DynamoDB) Put(ctx context.Context, class, key string, i interface{}) error {
	data, err := json.Marshal(i)
	if err != nil {
		return err
	}
	put := &dynamodb.PutItemInput{
		TableName: aws.String(table),
		Item: map[string]*dynamodb.AttributeValue{
			"body":  {B: data},
			"key":   {S: aws.String(key)},
			"class": {S: aws.String(class)},
		},
	}
	_, err = db.Client.PutItemWithContext(ctx, put)
	return err
}

func (db *DynamoDB) Keys(ctx context.Context, class string) ([]string, error) {
	query := &dynamodb.QueryInput{
		TableName:              aws.String(table),
		KeyConditionExpression: aws.String("class = :class"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":class": {S: aws.String(class)},
		},
		ProjectionExpression: aws.String("class"),
	}
	res, err := db.Client.QueryWithContext(ctx, query)
	if err != nil {
		return nil, err
	}
	keys := []string{}
	for _, item := range res.Items {
		keys = append(keys, *item["class"].S)
	}
	return keys, nil
}

func (db *DynamoDB) Get(ctx context.Context, class, key string, i interface{}) error {
	get := &dynamodb.GetItemInput{
		ConsistentRead: aws.Bool(true),
		TableName:      aws.String(table),
		Key: map[string]*dynamodb.AttributeValue{
			"key":   {S: aws.String(key)},
			"class": {S: aws.String(class)},
		},
	}
	res, err := db.Client.GetItemWithContext(ctx, get)
	data := res.Item["body"].B
	err = json.Unmarshal(data, i)
	if err != nil {
		return err
	}
	return nil
}

func (db *DynamoDB) Del(ctx context.Context, class, key string) error {
	get := &dynamodb.DeleteItemInput{
		TableName: aws.String(table),
		Key: map[string]*dynamodb.AttributeValue{
			"key":   {S: aws.String(key)},
			"class": {S: aws.String(class)},
		},
	}
	_, err := db.Client.DeleteItemWithContext(ctx, get)
	return err
}
