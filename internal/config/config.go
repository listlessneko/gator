package config

import (
	"encoding/json"
	"os"
)

const (
	configFileName = "/.gatorconfig.json"
)

type Config struct {
	DBURL           string `json:"db_url"`
	CurrentUserName string `json:"current_user_name"`
}

func getConfigFilePath() (string, error) {
	filePath, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	configFilePath := filePath + configFileName
	return configFilePath, nil
}

func Read() (Config, error) {
	var cfg Config

	filePath, err := getConfigFilePath()
	if err != nil {
		return cfg, err
	}
	data, err := os.ReadFile(filePath)
	if !(os.IsNotExist(err)) {
		if err == nil {
			err = json.Unmarshal(data, &cfg)
		}
		if err != nil {
			return cfg, err
		}
	}
	return cfg, nil
}

func (c Config) SetUser(username string) error {
	c.CurrentUserName = username
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}
	configFilePath, err := getConfigFilePath()
	if err != nil {
		return err
	}
	err = os.WriteFile(configFilePath, data, 0644)
	if err != nil {
		return err
	}
	return nil
}
