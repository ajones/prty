package stats

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"time"

	"github.com/inburst/prty/config"
	"github.com/inburst/prty/datasource"
	"github.com/inburst/prty/logger"
)

type PRStatsV1 struct {
	Event string `json:"Event"`

	OrgName  string `json:"OrgName"`
	RepoName string `json:"RepoName"`

	IAmAuthor                  bool
	AuthorIsTeammate           bool
	AuthorIsBot                bool
	HasChangesAfterLastComment bool
	HasCommentsFromMe          bool
	LastCommentFromMe          bool
	IsApproved                 bool
	IsAbandoned                bool
	IsDraft                    bool
	Additions                  int
	Deletions                  int
	CodeDelta                  int
	TimeSinceLastComment       time.Duration
	TimeSinceLastCommit        time.Duration
	TimeSinceFirstCommit       time.Duration
	PreviouslyOpened           bool
}

type Stats struct {
	StatsVersion int `yaml:"StatsVersion"`

	LifetimePRViews int `yaml:"LifetimePRViews"`
	LifetimePROpens int `yaml:"LifetimePROpens"`

	PRViewsPerAuthor map[string]int `yaml:"PRViewsPerAuthor"`
	PROpensPerAuthor map[string]int `yaml:"PROpensPerAuthor"`
}

type TrainingDataV1 struct {
	Version int `yaml:"Version"`

	PRViewHistory []PRStatsV1 `yaml:"PRViewHistory"`
	PROpenHistory []PRStatsV1 `yaml:"PROpenHistory"`
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

	// initialize maps if they are nil
	if s.PRViewsPerAuthor == nil {
		s.PRViewsPerAuthor = make(map[string]int)
	}
	if s.PROpensPerAuthor == nil {
		s.PROpensPerAuthor = make(map[string]int)
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

func (s *Stats) OnViewPR(pr *datasource.PullRequest) {
	// always save after edits
	defer s.SaveToFile()

	// initialize the count for this author if they hae not been seen before
	if _, ok := s.PRViewsPerAuthor[pr.Author]; !ok {
		s.PRViewsPerAuthor[pr.Author] = 0
	}

	s.LifetimePRViews += 1
	s.PRViewsPerAuthor[pr.Author] += 1

	go AppendEventToTrainingData("view", pr)
}

func (s *Stats) OnOpenPR(pr *datasource.PullRequest) {
	// always save after edits
	defer s.SaveToFile()

	// initialize the count for this author if they hae not been opened before
	if _, ok := s.PROpensPerAuthor[pr.Author]; !ok {
		s.PROpensPerAuthor[pr.Author] = 0
	}

	s.LifetimePROpens += 1
	s.PROpensPerAuthor[pr.Author] += 1

	go AppendEventToTrainingData("open", pr)
}

func (s *Stats) OnPassPR(pr *datasource.PullRequest) {
	// always save after edits
	defer s.SaveToFile()

	go AppendEventToTrainingData("pass", pr)
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

func AppendEventToTrainingData(eventName string, pr *datasource.PullRequest) error {
	trainingDataPath, err := config.GetTrainingFilePath()
	if err != nil {
		logger.Shared().Printf("err locating training data path %s\n", err)
		return err
	}

	eventData := PRStatsV1{
		Event: eventName,

		OrgName:  pr.OrgName,
		RepoName: pr.RepoName,

		IAmAuthor:                  pr.IAmAuthor,
		AuthorIsTeammate:           pr.AuthorIsTeammate,
		AuthorIsBot:                pr.AuthorIsBot,
		HasChangesAfterLastComment: pr.HasChangesAfterLastComment,
		HasCommentsFromMe:          pr.HasCommentsFromMe,
		LastCommentFromMe:          pr.LastCommentFromMe,
		IsApproved:                 pr.IsApproved,
		IsAbandoned:                pr.IsAbandoned,
		IsDraft:                    pr.IsDraft,
		Additions:                  pr.Additions,
		Deletions:                  pr.Deletions,
		CodeDelta:                  pr.CodeDelta,
		TimeSinceLastComment:       pr.TimeSinceLastComment,
		TimeSinceLastCommit:        pr.TimeSinceLastCommit,
		TimeSinceFirstCommit:       pr.TimeSinceFirstCommit,
		PreviouslyOpened:           pr.ViewedAt != nil,
	}

	data, err := json.Marshal(&eventData)
	if err != nil {
		logger.Shared().Printf("err marshaling training data event %s\n", err)
		return err
	}

	// append to the file
	f, err := os.OpenFile(trainingDataPath, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0666)
	if err != nil {
		logger.Shared().Printf("err opening training data file %s\n", err)
		return err
	}
	defer f.Close()

	_, err = f.WriteString(string(data) + "\n")
	if err != nil {
		logger.Shared().Printf("err writing training data event %s\n", err)
		return err
	}
	return nil
}
