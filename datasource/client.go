package datasource

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/google/go-github/v34/github"
	"github.com/inburst/prty/config"
	"golang.org/x/oauth2"
)

type Datasource struct {
	statusChan            chan<- string
	prUpdateChan          chan<- *PullRequest
	remainingRequestsChan chan<- github.Rate

	config *config.Config

	allPRs []*PullRequest
}

var sharedGithubClient *github.Client

func sharedClient() *github.Client {
	if sharedGithubClient == nil {
		ctx := context.Background()
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: os.Getenv("GITHUB_ACCESS_TOKEN")},
		)
		tc := oauth2.NewClient(ctx, ts)

		sharedGithubClient = github.NewClient(tc)
	}

	return sharedGithubClient
}

func New(c *config.Config) *Datasource {
	ds := &Datasource{}
	ds.config = c
	return ds
}

func (ds *Datasource) writeErrorStatus(err error) {
	ds.statusChan <- fmt.Sprintf("ERROR: %s", err)
}

func (ds *Datasource) writeStatus(message string) {
	ds.statusChan <- message
}

func (ds *Datasource) SetStatusChan(statusChan chan<- string) {
	ds.statusChan = statusChan
}

func (ds *Datasource) SetRemainingRequestsChan(remainingRequestsChan chan<- github.Rate) {
	ds.remainingRequestsChan = remainingRequestsChan
}

func (ds *Datasource) SetPRUpdateChan(prUpdateChan chan<- *PullRequest) {
	ds.prUpdateChan = prUpdateChan
}

func (ds *Datasource) LoadLocalCache() {
	// load in previous prs and emit them to the views
	ds.allPRs = ds.loadSaveFile()
	for _, pr := range ds.allPRs {
		ds.prUpdateChan <- pr
	}
}

func (ds *Datasource) loadSaveFile() []*PullRequest {
	prs := []*PullRequest{}
	homeDirName, _ := os.UserHomeDir()
	data, err := ioutil.ReadFile(fmt.Sprintf("%s/.prty/prs.json", homeDirName))
	if err != nil {
		println(fmt.Sprintf("%s", err))
		return prs
	}
	err = json.Unmarshal(data, &prs)
	println(fmt.Sprintf("%s", err))
	return prs
}

func (ds *Datasource) saveToFile() {
	homeDirName, _ := os.UserHomeDir()
	file, _ := json.MarshalIndent(ds.allPRs, "", " ")
	_ = ioutil.WriteFile(fmt.Sprintf("%s/.prty/prs.json", homeDirName), file, 0644)
}

func (ds *Datasource) RefreshData() {
	ds.writeStatus("fetching orgs...")

	/*
		ds.allPRs = append(ds.allPRs, &PullRequest{})
		ds.allPRs = append(ds.allPRs, &PullRequest{})
		ds.saveToFile()
		return
	*/

	orgs, err := GetAllOrgs()
	if err != nil {
		ds.writeErrorStatus(err)
		return
	}
	ds.writeStatus("gotem ")

	for i := range orgs {
		if orgs[i].Login != nil {
			orgName := *orgs[i].Login
			if listContains(ds.config.OrgBlacklist, orgName) {
				continue
			}
			// if the whitelist is empty or this oprg is whitelisted
			if len(ds.config.OrgWhitelist) == 0 || listContains(ds.config.OrgWhitelist, orgName) {

				ds.writeStatus(fmt.Sprintf("%s fetching repos...", orgName))
				repos, err := GetAllReposForOrg(orgName)
				if err != nil {
					ds.writeErrorStatus(err)
					println(fmt.Sprintf("%s\n", err))
					return
				}

				for _, repo := range repos {
					repoName := *repo.Name
					go ds.refreshRepo(orgName, repoName)
				}
			}
		}
	}
}

func (ds *Datasource) refreshRepo(orgName string, repoName string) {
	ds.writeStatus(fmt.Sprintf("%s/%s fetching prs...", orgName, repoName))
	prs, err := ds.GetAllPullsForRepoInOrg(orgName, repoName)
	if err != nil {
		ds.writeErrorStatus(err)
		println(fmt.Sprintf("%s\n", err))
		return
	}

	for _, ghpr := range prs {
		go ds.buildPr(orgName, repoName, ghpr)
	}
}

func (ds *Datasource) buildPr(orgName string, repoName string, ghpr *github.PullRequest) {
	var existingPr *PullRequest
	for _, pr := range ds.allPRs {
		if pr.PR.GetID() == ghpr.GetID() {
			existingPr = pr
			break
		}
	}

	if existingPr != nil {
		ds.UpdateExistingPr(orgName, repoName, existingPr, ghpr)
	} else {
		newPR, err := ds.BuildPullRequest(orgName, repoName, ghpr)
		if err != nil {
			ds.writeErrorStatus(err)
			println(fmt.Sprintf("%s\n", err))
			return
		}
		ds.allPRs = append(ds.allPRs, newPR)
		ds.saveToFile()
		ds.prUpdateChan <- newPR
		ds.statusChan <- "" // clear status after each PR
	}
}
