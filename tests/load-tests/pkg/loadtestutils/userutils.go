package loadtestutils

import "encoding/json"
import "os"
import "path/filepath"

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
	filePath = filepath.Clean(filePath)
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
