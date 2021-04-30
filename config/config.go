package config

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	"gopkg.in/yaml.v3"
)

const PRTYVersion = "0.0.8"
const AppCacheDirName = ".prty"
const ConfFileName = "conf.yaml"
const StatsFileName = "stats.json"
const PRCacheFileName = "prs.json"
const LogFileName = "prty.log"
const DefaultGithubToken = "token with repo read permission"
const DefaultGithubUserName = "your github username"

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

	filePath, err := GetConfigFilePath()
	if err != nil {
		return nil, err
	}

	c := &Config{}
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal(data, c)
	if err != nil {
		return nil, err
	}

	if err = validate(c); err != nil {
		return nil, err
	}

	return c, nil
}

func validate(c *Config) error {
	filePath, err := GetConfigFilePath()
	if err != nil {
		return err
	}

	// detect default configuration
	if c.GithubUsername == DefaultGithubUserName || len(c.GithubUsername) == 0 ||
		c.GithubAccessToken == DefaultGithubToken || len(c.GithubAccessToken) == 0 {
		msgFormat := "Please set up configuration at %s\nYou must input your github username and generate a github access token with `org:read` and `repo` permissions."
		return errors.New(fmt.Sprintf(msgFormat, filePath))
	}

	if c.AbandonedAgeDays < 0 {
		errFormat := "AbandonedAgeDays must be a value greater than or equal to 0, currently [%d]\n Config file can be found at %s\n"
		return errors.New(fmt.Sprintf(errFormat, c.AbandonedAgeDays, filePath))
	}
	return nil
}

func checkAndCreateConfigFile() error {
	err := PrepApplicationCacheFolder()
	if err != nil {
		return err
	}

	confPath, err := GetConfigFilePath()
	if err != nil {
		return err
	}

	if _, err := os.Stat(confPath); os.IsNotExist(err) {
		// some default values
		blankConfig := &Config{
			ConfigVersion:     1,
			GithubAccessToken: DefaultGithubToken,
			GithubUsername:    DefaultGithubUserName,
			AbandonedAgeDays:  21,
			RefreshOnStart:    true,
		}
		err = blankConfig.SaveToFile()
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Config) SaveToFile() error {
	confPath, err := GetConfigFilePath()
	if err != nil {
		return err
	}

	var b bytes.Buffer
	yamlEncoder := yaml.NewEncoder(&b)
	yamlEncoder.SetIndent(2)
	yamlEncoder.Encode(&c)
	err = ioutil.WriteFile(confPath, b.Bytes(), 0644)
	return err
}

func getUserHomePathPath() (string, error) {
	homeDirName, err := os.UserHomeDir()
	return homeDirName, err
}

func buildScopedPathFor(filename string) (string, error) {
	appCachePath, err := GetApplicationCachePath()
	if err != nil {
		return "", err
	}
	filePath := fmt.Sprintf("%s/%s", appCachePath, filename)
	return filePath, nil
}

func PrepApplicationCacheFolder() error {
	appCachePath, err := GetApplicationCachePath()
	if err != nil {
		return err
	}
	os.Mkdir(appCachePath, 0755)
	return nil
}

func GetApplicationCachePath() (string, error) {
	homeDirPath, err := getUserHomePathPath()
	if err != nil {
		return "", err
	}
	filePath := fmt.Sprintf("%s/%s", homeDirPath, AppCacheDirName)
	return filePath, nil
}

func GetConfigFilePath() (string, error) {
	return buildScopedPathFor(ConfFileName)
}

func GetStatsFilePath() (string, error) {
	return buildScopedPathFor(StatsFileName)
}

func GetPRCacheFilePath() (string, error) {
	return buildScopedPathFor(PRCacheFileName)
}

func GetLogFilePath() (string, error) {
	return buildScopedPathFor(LogFileName)
}
