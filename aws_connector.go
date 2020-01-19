package main

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/expression"
	"github.com/segmentio/ksuid"
	"log"
)

type awsConnector struct {
}

func (conn *awsConnector) GetUsers() []User {
	var users []User
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))
	dbc := dynamodb.New(sess)

	proj := expression.NamesList(expression.Name("name"), expression.Name("id"))
	expr, err := expression.NewBuilder().WithProjection(proj).Build()

	if err != nil {
		log.Printf("Failed to create exptession: %s", err.Error())
		return users
	}

	params := &dynamodb.ScanInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		ProjectionExpression:      expr.Projection(),
		TableName:                 aws.String("BoardGameUsers"),
	}

	result, err := dbc.Scan(params)
	if err != nil {
		log.Printf("Failed to scan: %s", err.Error())
		return users
	}

	for _, i := range result.Items {
		user := User{}

		err = dynamodbattribute.UnmarshalMap(i, &user)
		if err != nil {
			log.Printf("Failed to unmarshal: %s", err.Error())
			continue
		}
		users = append(users, user)
	}

	return users
}

func (conn *awsConnector) AddUser(user *User) error {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))

	// Create DynamoDB client
	svc := dynamodb.New(sess)
	user.ID = ksuid.New().String()
	av, err := dynamodbattribute.MarshalMap(user)
	if err != nil {
		log.Printf("Failed to create item: %s", err.Error())
		return err
	}
	input := &dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String("BoardGameUsers"),
	}

	_, err = svc.PutItem(input)
	if err != nil {
		log.Printf("Failed to add item: %s", err.Error())
		return err
	}
	return nil
}
