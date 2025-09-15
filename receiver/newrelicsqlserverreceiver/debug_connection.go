package main

import (
	"fmt"
	"time"
	
	newrelicsqlserverreceiver "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicsqlserverreceiver"
)

func main() {
	// Azure AD config test
	azureConfig := &newrelicsqlserverreceiver.Config{in

import (
"fmt"
"time"
)

func main() {
	// Azure AD config test
	azureConfig := &Config{
		Hostname:     "sqlserver.database.windows.net",
		Port:         "1433",
		ClientID:     "client-id-123",
		TenantID:     "tenant-id-456",
		ClientSecret: "client-secret-789",
		Timeout:      30 * time.Second,
	}
	
	azureURL := azureConfig.CreateAzureADConnectionURL("master")
	fmt.Println("Azure AD URL:", azureURL)
	
	// Windows auth config test  
	windowsConfig := &newrelicsqlserverreceiver.Config{
		Hostname: "localhost",
		Port:     "1433",
		Timeout:  30 * time.Second,
	}
	
	windowsURL := windowsConfig.CreateConnectionURL("master")
	fmt.Println("Windows URL:", windowsURL)
}
