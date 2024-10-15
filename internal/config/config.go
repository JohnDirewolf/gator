package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

const configFileName = ".gatorconfig.json"
const localPath = "/workspace/github.com/JohnDirewolf/gator/"

type Config struct {
	DbUrl           string `json:"db_url"`
	CurrentUserName string `json:"current_user_name"`
}

type MyTest struct {
	Check    bool
	A_string string
}

func getConfigFilePath() (string, error) {
	homePath, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return homePath + "/" + configFileName, nil
	//fullFilePath = "./" + configFileName - This was a local file test with relative path.
}

func Read() (Config, error) {
	//This reads the config file .gatorconfig.json getting the data connection URL and returns it.

	fullFilePath, err := getConfigFilePath()
	if err != nil {
		return Config{}, errors.New(fmt.Sprintf("Error getting Home: %v", err))
	}

	// Open the JSON file
	file, err := os.Open(fullFilePath)
	if err != nil {
		return Config{}, errors.New(fmt.Sprintf("Error opening file: %v", err))
	}
	defer file.Close()

	// Decode the JSON data
	decoder := json.NewDecoder(file)
	var configData Config
	err = decoder.Decode(&configData)
	if err != nil {
		return Config{}, errors.New(fmt.Sprintf("Error decoding JSON: %v", err))
	}

	return configData, nil
}

func (u Config) SetUser(userName string) error {

	fullFilePath, err := getConfigFilePath()
	if err != nil {
		return errors.New(fmt.Sprintf("Error getting Home: %v", err))
	}

	//This sets the user in the JSON structure and then writes it back to the .gatorconfig.json file.
	u.CurrentUserName = userName

	// Marshal the data into JSON format
	jsonData, err := json.Marshal(u)
	if err != nil {
		return errors.New(fmt.Sprintf("Error marshaling JSON: %v", err))
	}

	// Write the JSON data to a file
	err = os.WriteFile(fullFilePath, jsonData, 0644)
	if err != nil {
		return errors.New(fmt.Sprintf("Error writing to file: %v", err))
	}

	return nil
}
