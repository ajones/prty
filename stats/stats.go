package stats

import (
	"encoding/json"
	"io/ioutil"
	"os"

	"github.com/inburst/prty/config"
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

	filePath, err := config.GetStatsFilePath()
	if err != nil {
		return nil, err
	}

	s := &Stats{}
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(data, &s)
	if err != nil {
		return nil, err
	}

	return s, nil
}

func checkAndCreateStatsFile() error {
	err := config.PrepApplicationCacheFolder()
	if err != nil {
		return err
	}

	statsPath, err := config.GetStatsFilePath()
	if err != nil {
		return err
	}

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
	statsPath, err := config.GetStatsFilePath()
	if err != nil {
		return err
	}

	file, err := json.MarshalIndent(s, "", " ")
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(statsPath, file, 0644)
	return err
}
