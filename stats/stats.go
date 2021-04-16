package stats

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"

	"gopkg.in/yaml.v3"
)

type Stats struct {
	StatsVersion int `yaml:"StatsVersion"`

	LifetimePROpens  int            `yaml:"LifetimePROpens"`
	PROpensPerAuthor map[string]int `yaml:"PROpensPerAuthor"`
}

func LoadStats() (*Stats, error) {
	err := checkAndCreateStatsFile()
	if err != nil {
		return nil, err
	}

	homeDirName, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	c := &Stats{}
	yamlFile, err := ioutil.ReadFile(fmt.Sprintf("%s/.prty/stats.yaml", homeDirName))
	if err != nil {
		println("yamlFile.Get err #%v ", err)
	}
	err = yaml.Unmarshal(yamlFile, c)
	if err != nil {
		println("Unmarshal: %v", err)
	}

	println("Loaded stats: ", fmt.Sprintf("%+v", c))

	return c, nil
}

func checkAndCreateStatsFile() error {
	homeDirName, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	dirPath := fmt.Sprintf("%s/.prty", homeDirName)
	os.Mkdir(dirPath, 0755)

	confPath := fmt.Sprintf("%s/stats.yaml", dirPath)
	if _, err := os.Stat(confPath); os.IsNotExist(err) {
		// some default values
		blankConfig := &Stats{
			StatsVersion: 1,
		}

		var b bytes.Buffer
		yamlEncoder := yaml.NewEncoder(&b)
		yamlEncoder.SetIndent(2)
		yamlEncoder.Encode(&blankConfig)
		err = ioutil.WriteFile(confPath, b.Bytes(), 0644)
		if err != nil {
			return err
		}
	}
	return nil
}
