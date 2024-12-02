package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-lambda-go/cfn"
	"github.com/aws/aws-lambda-go/lambda"
)

const (
	PropertyNameRootDatabaseConfig    = "RootDatabaseConfig"
	PropertyNameServiceDatabaseConfig = "ServiceDatabaseConfig"
)

type RootDatabaseConfig struct {
	Password             string `json:"password"`
	DBName               string `json:"dbname"`
	Engine               string `json:"engine"`
	Port                 int    `json:"port"`
	DBInstanceIdentifier string `json:"dbInstanceIdentifier"`
	Host                 string `json:"host"`
	Username             string `json:"username"`
	DisableTLS           bool   `json:"disableTLS"`
}

type ServiceDatabaseConfig struct {
	DBName   string `json:"dbname"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func pgDatabaseResource(ctx context.Context, event cfn.Event) (string, map[string]interface{}, error) {
	switch event.RequestType {
	case cfn.RequestCreate:
		return createDatabase(ctx, event)
	case cfn.RequestUpdate:
		return updateDatabase(ctx, event)
	case cfn.RequestDelete:
		return deleteDatabase(ctx, event)
	}

	return "", nil, fmt.Errorf("unknown request type %s", event.RequestType)

}

func createDatabase(_ context.Context, event cfn.Event) (string, map[string]interface{}, error) {
	rootConfig, serviceConfig, err := getProperties(event.ResourceProperties)
	if err != nil {
		return "", nil, err
	}
	// For simplicity we require the dbname and username to match
	if serviceConfig.DBName != serviceConfig.Username {
		return "", nil, fmt.Errorf("service dbname and username must match")
	}

	fmt.Println("Connecting to database: ", rootConfig.DBInstanceIdentifier)
	fmt.Println("Creating database: ", serviceConfig.DBName)
	fmt.Println("Creating user: ", serviceConfig.Username)

	physicalResourceID := rootConfig.DBInstanceIdentifier + "/" + serviceConfig.DBName

	return physicalResourceID, nil, nil

}

func updateDatabase(ctx context.Context, event cfn.Event) (string, map[string]interface{}, error) {
	rootConfig, serviceConfig, err := getProperties(event.ResourceProperties)
	if err != nil {
		return "", nil, err
	}

	oldRootConfig, oldServiceConfg, err := getProperties(event.OldResourceProperties)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get old properties: %w", err)
	}

	// For simplicity we require the dbname and username to match
	if serviceConfig.DBName != serviceConfig.Username {
		return "", nil, fmt.Errorf("service dbname and username must match")
	}

	// If the DBInstanceIdentifier or DBName has changed, we need to create a new database
	// We don't have to worry about cleaning up the old database, cloudformation
	// will call delete on the old database when we return a new physical resource ID
	if oldRootConfig.DBInstanceIdentifier != rootConfig.DBInstanceIdentifier ||
		oldServiceConfg.DBName != serviceConfig.DBName {
		return createDatabase(ctx, event)
	}

	fmt.Println("Connecting to database: ", rootConfig.DBInstanceIdentifier)
	fmt.Println("updating password for user: ", serviceConfig.Username)
	physicalResourceID := rootConfig.DBInstanceIdentifier + "/" + serviceConfig.DBName

	return physicalResourceID, nil, nil
}

func deleteDatabase(_ context.Context, event cfn.Event) (string, map[string]interface{}, error) {
	rootConfig, serviceConfig, err := getProperties(event.ResourceProperties)
	if err != nil {
		return "", nil, err
	}

	fmt.Println("Connecting to database: ", rootConfig.DBInstanceIdentifier)
	fmt.Println("Deleting database: ", serviceConfig.DBName)
	fmt.Println("Deleting user: ", serviceConfig.Username)

	return "", nil, nil
}

func getProperties(properties map[string]interface{}) (*RootDatabaseConfig, *ServiceDatabaseConfig, error) {
	rootConfig := &RootDatabaseConfig{}
	serviceConfig := &ServiceDatabaseConfig{}

	rootConfigStr, ok := properties[PropertyNameRootDatabaseConfig].(string)
	if !ok {
		return nil, nil, fmt.Errorf("RootDatabaseConfig must be a string")
	}

	serviceConfigStr, ok := properties[PropertyNameServiceDatabaseConfig].(string)
	if !ok {
		return nil, nil, fmt.Errorf("ServiceDatabaseConfig must be a string")
	}

	if err := json.Unmarshal([]byte(rootConfigStr), rootConfig); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal root config: %w", err)
	}

	if err := json.Unmarshal([]byte(serviceConfigStr), serviceConfig); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal service config: %w", err)
	}

	return rootConfig, serviceConfig, nil
}

func main() {
	lambda.Start(cfn.LambdaWrap(pgDatabaseResource))
}
