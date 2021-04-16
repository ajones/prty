package datasource

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"sync"

	"github.com/google/go-github/v34/github"
	"github.com/inburst/prty/config"
	"golang.org/x/oauth2"
)

type Datasource struct {
	statusChan            chan<- string
	prUpdateChan          chan<- *PullRequest
	remainingRequestsChan chan<- github.Rate

	config *config.Config

	allPRs    map[string]*PullRequest
	cachedPRs map[string]*PullRequest

	mutex sync.RWMutex
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

func (ds *Datasource) loadSaveFile() map[string]*PullRequest {
	prs := map[string]*PullRequest{}
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
	ds.mutex.Lock()
	homeDirName, _ := os.UserHomeDir()
	file, _ := json.MarshalIndent(ds.allPRs, "", " ")
	_ = ioutil.WriteFile(fmt.Sprintf("%s/.prty/prs.json", homeDirName), file, 0644)
	ds.mutex.Unlock()
}

func (ds *Datasource) RefreshData() {
	ds.cachedPRs = ds.allPRs
	ds.allPRs = map[string]*PullRequest{}

	ds.writeStatus("fetching orgs...")
	orgs, err := GetAllOrgs()
	if err != nil {
		ds.writeErrorStatus(err)
		return
	}

	for i := range orgs {
		if orgs[i].Login != nil {
			orgName := *orgs[i].Login
			if listContains(ds.config.OrgBlacklist, orgName) {
				continue
			}

			if (len(ds.config.OrgWhitelist) == 0 || listContains(ds.config.OrgWhitelist, orgName)) &&
				(len(ds.config.OrgBlacklist) == 0 || !listContains(ds.config.OrgBlacklist, orgName)) {
				ds.writeStatus(fmt.Sprintf("%s fetching repos...", orgName))
				repos, err := GetAllReposForOrg(orgName)
				if err != nil {
					ds.writeErrorStatus(err)
					println(fmt.Sprintf("%s\n", err))
					return
				}

				for _, repo := range repos {
					repoName := *repo.Name
					if (len(ds.config.RepoWhitelist) == 0 || listContains(ds.config.RepoWhitelist, orgName)) &&
						(len(ds.config.RepoBlacklist) == 0 || !listContains(ds.config.RepoBlacklist, orgName)) {
						go ds.refreshRepo(orgName, repoName)
					}
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
	if existingPr, ok := ds.cachedPRs[ghpr.GetNodeID()]; ok {
		ds.UpdateExistingPr(orgName, repoName, existingPr, ghpr)
		ds.mutex.Lock()
		ds.allPRs[existingPr.PR.GetNodeID()] = existingPr
		ds.mutex.Unlock()
		ds.prUpdateChan <- existingPr
	} else {
		newPR, err := ds.BuildPullRequest(orgName, repoName, ghpr)
		if err != nil {
			ds.writeErrorStatus(err)
			println(fmt.Sprintf("%s\n", err))
			return
		}
		ds.mutex.Lock()
		ds.allPRs[newPR.PR.GetNodeID()] = newPR
		ds.mutex.Unlock()
		ds.prUpdateChan <- newPR
	}

	ds.saveToFile()
	ds.statusChan <- "" // clear status after each PR
}
