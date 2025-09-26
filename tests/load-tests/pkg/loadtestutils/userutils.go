package loadtestutils

import "encoding/json"
import "fmt"
import "os"
import "path/filepath"

// Represents a user in the list of precreated users (e.g. Stage 'users.json')
type User struct {
	Namespace string `json:"namespace"`
	Token     string `json:"token"`
	APIURL    string `json:"apiurl"`
	Verified  bool   `json:"verified"`
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

	// Some sanity checks
	if len(users) == 0 {
		return nil, fmt.Errorf("Loaded %s but no users in there", filePath)
	}
	if users[0].APIURL == "" || users[0].Token == "" || users[0].Namespace == "" {
		return nil, fmt.Errorf("Loaded %s but some expected field missing in first user", filePath)
	}
	return users, nil
}
