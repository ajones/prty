package stats

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/inburst/prty/datasource"
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

	data, err := ioutil.ReadFile(fmt.Sprintf("%s/.prty/stats.json", homeDirName))
	if err != nil {
		println("read error %v ", err)
	}
	err = json.Unmarshal(data, &c)
	if err != nil {
		println("unmarshall err %v ", err)
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

	statsPath := fmt.Sprintf("%s/stats.json", dirPath)
	if _, err := os.Stat(statsPath); os.IsNotExist(err) {
		// some default values
		blankConfig := &Stats{
			StatsVersion:     1,
			PROpensPerAuthor: make(map[string]int),
		}
		blankConfig.SaveToFile()
	}
	return nil
}

func (s *Stats) OnViewedPR(pr *datasource.PullRequest) {
	s.LifetimePROpens += 1

	if count, ok := s.PROpensPerAuthor[pr.Author]; !ok {
		s.PROpensPerAuthor[pr.Author] = 1
	} else {
		s.PROpensPerAuthor[pr.Author] = count + 1
	}
	s.SaveToFile()
}

func (s *Stats) SaveToFile() error {
	homeDirName, _ := os.UserHomeDir()
	file, _ := json.MarshalIndent(s, "", " ")
	err := ioutil.WriteFile(fmt.Sprintf("%s/.prty/stats.json", homeDirName), file, 0644)
	return err
}
