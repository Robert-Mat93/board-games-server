package main

import (
	"errors"
)

type Connector interface {
	GetUsers() []User
	AddUser(*User) error
}

type ConnectorType int

const (
	DynamoDB = iota
)

func NewConnector(connectorType ConnectorType) (Connector, error) {
	switch connectorType {
	case DynamoDB:
		return &awsConnector{}, nil
	}
	return nil, errors.New("Invalid connector type.")
}
