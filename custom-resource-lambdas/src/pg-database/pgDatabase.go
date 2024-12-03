package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-lambda-go/cfn"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
)

const (
	PropertyNameAdminSecretArn   = "AdminSecretArn"
	PropertyNameServiceSecretArn = "ServiceSecretArn"

	CustomTypeNamePgDatabase = "Custom::PgDatabase"
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

type CustomResourceManager struct {
	secretsManager *secretsmanager.SecretsManager
}

func (c *CustomResourceManager) HandleCustomResourceEvent(ctx context.Context, event cfn.Event) (string, map[string]interface{}, error) {
	switch event.ResourceType {
	case CustomTypeNamePgDatabase:
		switch event.RequestType {
		case cfn.RequestCreate:
			return c.createDatabase(ctx, event)
		case cfn.RequestUpdate:
			return c.updateDatabase(ctx, event)
		case cfn.RequestDelete:
			return c.deleteDatabase(ctx, event)
		}
	}

	return "", nil, fmt.Errorf("unknown request type or resource: eventType=%s resourceType=%s", event.RequestType, event.ResourceType)

}

func (c *CustomResourceManager) createDatabase(ctx context.Context, event cfn.Event) (string, map[string]interface{}, error) {
	rootConfig, serviceConfig, err := c.getProperties(ctx, event.ResourceProperties)
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

func (c *CustomResourceManager) updateDatabase(ctx context.Context, event cfn.Event) (string, map[string]interface{}, error) {
	rootConfig, serviceConfig, err := c.getProperties(ctx, event.ResourceProperties)
	if err != nil {
		return "", nil, err
	}

	oldRootConfig, oldServiceConfg, err := c.getProperties(ctx, event.OldResourceProperties)
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
		return c.createDatabase(ctx, event)
	}

	fmt.Println("Connecting to database: ", rootConfig.DBInstanceIdentifier)
	fmt.Println("updating password for user: ", serviceConfig.Username)
	physicalResourceID := rootConfig.DBInstanceIdentifier + "/" + serviceConfig.DBName

	return physicalResourceID, nil, nil
}

func (c *CustomResourceManager) deleteDatabase(ctx context.Context, event cfn.Event) (string, map[string]interface{}, error) {
	rootConfig, serviceConfig, err := c.getProperties(ctx, event.ResourceProperties)
	if err != nil {
		return "", nil, err
	}

	fmt.Println("Connecting to database: ", rootConfig.DBInstanceIdentifier)
	fmt.Println("Deleting database: ", serviceConfig.DBName)
	fmt.Println("Deleting user: ", serviceConfig.Username)

	return "", nil, nil
}

func (c *CustomResourceManager) getProperties(ctx context.Context, properties map[string]interface{}) (*RootDatabaseConfig, *ServiceDatabaseConfig, error) {
	rootConfig := RootDatabaseConfig{}
	serviceConfig := ServiceDatabaseConfig{}

	adminSecretArn, ok := properties[PropertyNameAdminSecretArn].(string)
	if !ok {
		return nil, nil, fmt.Errorf("RootDatabaseConfig must be a string")
	}

	serviceSecretArn, ok := properties[PropertyNameServiceSecretArn].(string)
	if !ok {
		return nil, nil, fmt.Errorf("ServiceDatabaseConfig must be a string")
	}

	err := c.getSecret(ctx, adminSecretArn, &rootConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get admin secret: %w", err)
	}

	err = c.getSecret(ctx, serviceSecretArn, &serviceConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get service secret: %w", err)
	}

	return &rootConfig, &serviceConfig, nil
}

func (c *CustomResourceManager) getSecret(ctx context.Context, secretArn string, dest any) error {

	input := &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretArn),
	}

	result, err := c.secretsManager.GetSecretValueWithContext(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to get secret: %w", err)
	}

	if result.SecretString == nil {
		return fmt.Errorf("secret string is nil")
	}

	err = json.Unmarshal([]byte(*result.SecretString), dest)
	if err != nil {
		return fmt.Errorf("failed to unmarshal secret: %w", err)
	}

	return nil
}

func main() {
	awsSession, err := session.NewSession()
	if err != nil {
		panic(err)
	}
	svc := secretsmanager.New(awsSession)

	c := &CustomResourceManager{
		secretsManager: svc,
	}

	lambda.Start(cfn.LambdaWrap(c.HandleCustomResourceEvent))
}
