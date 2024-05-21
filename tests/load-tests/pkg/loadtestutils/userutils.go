package loadtestutils

import (
	"encoding/json"
	"errors"
	//	"net/url"
	"os"
	//	"time"
	//	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
)

// Represents a user in the list of precreated users (e.g. Stage 'users.json')
type User struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Token    string `json:"token"`
	SSOURL   string `json:"ssourl"`
	APIURL   string `json:"apiurl"`
	Verified bool   `json:"verified"`
}

// Load 'users.json' into a slice of User structs
func LoadStageUsers(filePath string) ([]User, error) {
	jsonData, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	// Parse JSON into an array of User objects
	var users []User
	err = json.Unmarshal(jsonData, &users)
	if err != nil {
		return nil, err
	}
	return users, nil
}

func SelectUsers(userList []User, numberOfUsers, threadCount, maxUsers int) ([]User, error) {
	// Check if the requested number of users exceeds the maximum
	if numberOfUsers*threadCount > maxUsers {
		return nil, errors.New("requested number of users exceeds maximum")
	}

	// Create a new list to store the selected users
	selectedUsers := make([]User, 0)

	// Iterate through the list and select Z users
	selectedCount := numberOfUsers * threadCount
	for i := 0; i < selectedCount; i++ {
		if i >= len(userList) {
			break // Stop if all users are selected
		}
		selectedUsers = append(selectedUsers, userList[i])
	}
	return selectedUsers, nil
}
