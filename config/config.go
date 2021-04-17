package config

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	"gopkg.in/yaml.v3"
)

const DefaultGithubToken = "token with repo read permission"
const DefaultGithubUserName = "Your github username"

type Config struct {
	ConfigVersion int `yaml:"ConfigVersion"`

	GithubAccessToken string   `yaml:"GithubAccessToken"`
	OrgWhitelist      []string `yaml:"OrgWhitelist"`
	OrgBlacklist      []string `yaml:"OrgBlacklist"`
	RepoWhitelist     []string `yaml:"RepoWhitelist"`
	RepoBlacklist     []string `yaml:"RepoBlacklist"`
	GithubUsername    string   `yaml:"GithubUsername"`
	BotUsernames      []string `yaml:"BotUsernames"`
	TeamUsernames     []string `yaml:"TeamUsernames"`
	AbandonedAgeDays  int      `yaml:"AbandonedAgeDays"`
	RefreshOnStart    bool     `yaml:"RefreshOnStart"`
}

func LoadConfig() (*Config, error) {
	err := checkAndCreateConfigFile()
	if err != nil {
		return nil, err
	}

	homeDirName, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	c := &Config{}
	filePath := fmt.Sprintf("%s/.prty/conf.yaml", homeDirName)
	yamlFile, err := ioutil.ReadFile(filePath)
	if err != nil {
		println("yamlFile.Get err #%v ", err)
	}
	err = yaml.Unmarshal(yamlFile, c)
	if err != nil {
		println("Unmarshal: %v", err)
	}

	// detect default configuration
	if c.GithubUsername == DefaultGithubUserName || c.GithubAccessToken == DefaultGithubToken {
		return nil, errors.New(fmt.Sprintf("Please set up configuration at %s", filePath))
	}

	return c, nil
}

func checkAndCreateConfigFile() error {
	homeDirName, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	dirPath := fmt.Sprintf("%s/.prty", homeDirName)
	os.Mkdir(dirPath, 0755)

	confPath := fmt.Sprintf("%s/conf.yaml", dirPath)
	if _, err := os.Stat(confPath); os.IsNotExist(err) {
		// some default values
		blankConfig := &Config{
			ConfigVersion:     1,
			GithubAccessToken: DefaultGithubToken,
			GithubUsername:    DefaultGithubUserName,
			AbandonedAgeDays:  21,
		}

		var b bytes.Buffer
		yamlEncoder := yaml.NewEncoder(&b)
		yamlEncoder.SetIndent(2) // this is what you're looking for
		yamlEncoder.Encode(&blankConfig)
		err = ioutil.WriteFile(confPath, b.Bytes(), 0644)
		if err != nil {
			return err
		}
	}
	return nil
}
