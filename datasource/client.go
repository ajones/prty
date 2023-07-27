package datasource

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"
	"sync"

	"github.com/google/go-github/v53/github"
	"github.com/inburst/prty/config"
	"github.com/inburst/prty/logger"
	"github.com/inburst/prty/tracking"
	"golang.org/x/oauth2"
)

type Datasource struct {
	statusChan            chan<- string
	prUpdateChan          chan<- *PullRequest
	remainingRequestsChan chan<- github.Rate

	config *config.Config

	allPRs              map[string]*PullRequest
	cachedPRs           map[string]*PullRequest
	currentlyRefreshing bool

	mutex sync.RWMutex
}

var sharedGithubClient *github.Client

func sharedClient() *github.Client {
	return sharedGithubClient
}

func InitSharedClient(tok string) error {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: tok},
	)
	tc := oauth2.NewClient(ctx, ts)

	sharedGithubClient = github.NewClient(tc)

	return nil
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
	logger.Shared().Printf("updating status %s\n", message)
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

func (ds *Datasource) IsCurrentlyRefreshingData() bool {
	return ds.currentlyRefreshing
}

// Supressed errors will cause the cache file to be emptied and rebuilt
// effectivly self healing from corrupt or invalid data
func (ds *Datasource) loadSaveFile() map[string]*PullRequest {
	prs := map[string]*PullRequest{}
	cacheFilePath, err := config.GetPRCacheFilePath()
	if err != nil {
		tracking.SendMetric("data.loadcache.patherror")
		return prs
	}
	data, err := ioutil.ReadFile(cacheFilePath)
	if err != nil {
		tracking.SendMetric("data.loadcache.readerror")
		return prs
	}
	err = json.Unmarshal(data, &prs)
	if err != nil {
		tracking.SendMetric("data.loadcache.unmarshallerror")
	}
	return prs
}

func (ds *Datasource) SaveToFile() {
	cacheFilePath, _ := config.GetPRCacheFilePath()
	ds.mutex.Lock()
	file, _ := json.MarshalIndent(ds.allPRs, "", " ")
	ds.mutex.Unlock()
	_ = ioutil.WriteFile(cacheFilePath, file, 0644)
}

func (ds *Datasource) RefreshData() {
	tracking.SendMetric("data.refresh")

	ds.currentlyRefreshing = true
	ds.cachedPRs = ds.allPRs
	ds.allPRs = map[string]*PullRequest{}

	ds.writeStatus("fetching users orgs...")
	orgs, err := GetAllOrgs()
	if err != nil {
		ds.writeErrorStatus(err)
		logger.Shared().Printf("%s\n", err)
		tracking.SendMetric("data.getorgs.error")
		ds.currentlyRefreshing = false
		return
	}

	ds.writeStatus("fetching users repos...")
	for i := range orgs {
		if orgs[i].Login != nil {
			orgName := *orgs[i].Login
			if listContains(ds.config.OrgBlacklist, orgName) {
				logger.Shared().Printf("skipping due to blacklist %s\n", orgName)
				continue
			}
			if len(ds.config.OrgWhitelist) != 0 && !listContains(ds.config.OrgWhitelist, orgName) {
				logger.Shared().Printf("skipping due to missing in whitelist %s\n", orgName)
				continue
			}

			ds.writeStatus(fmt.Sprintf("fetching repos for %s ...", orgName))

			// publicRepos := map[string]bool{}
			// for _, orgAndRepoName := range c.PublicRepos {
			// 	repoParts := strings.Split(orgAndRepoName, "/")
			// 	if len(repoParts) != 2 {
			// 		logger.Shared().Printf("invalid public repo name: %s\n", orgAndRepoName)
			// 		continue
			// 	}
			// 	orgName := repoParts[0]
			// 	publicRepos[]
			// }

			repos, err := GetAllReposForOrg(orgName)
			if err != nil {
				ds.writeErrorStatus(err)
				logger.Shared().Printf("%s\n", err)
				tracking.SendMetric("data.getrepos.error")
				ds.currentlyRefreshing = false
				return
			}

			for _, repo := range repos {
				repoName := *repo.Name
				if len(ds.config.RepoWhitelist) != 0 && !listContains(ds.config.RepoWhitelist, repoName) {
					logger.Shared().Printf("skipping due to whitelist %s:%s \n", orgName, repoName)
					continue
				}
				if len(ds.config.RepoBlacklist) != 0 && listContains(ds.config.RepoBlacklist, repoName) {
					logger.Shared().Printf("skipping due to blacklist %s:%s \n", orgName, repoName)
					continue
				}

				go ds.refreshRepo(orgName, repoName)
			}

		}
	}

	ds.writeStatus("fetching public repos...")
	for _, orgAndRepoName := range ds.config.PublicRepos {
		repoParts := strings.Split(orgAndRepoName, "/")
		if len(repoParts) != 2 {
			logger.Shared().Printf("invalid public repo name: %s\n", orgAndRepoName)
			continue
		}

		orgName := repoParts[0]
		repoName := repoParts[1]
		go ds.refreshRepo(orgName, repoName)
	}

	ds.currentlyRefreshing = false
	ds.writeStatus("refreshed")
}

func (ds *Datasource) refreshRepo(orgName string, repoName string) {
	ds.writeStatus(fmt.Sprintf("%s/%s fetching prs...", orgName, repoName))
	prs, err := ds.GetAllPullsForRepoInOrg(orgName, repoName)
	if err != nil {
		ds.writeErrorStatus(err)
		logger.Shared().Printf("%s\n", err)
		tracking.SendMetric("data.getpulls.error")
		return
	}

	for _, ghpr := range prs {
		go ds.buildPr(orgName, repoName, ghpr)
	}
}

func (ds *Datasource) buildPr(orgName string, repoName string, ghpr *github.PullRequest) {

	// Note: PRs that are sourced from the /list api will not contain all desired
	// fields including additions and deletions

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
			logger.Shared().Printf("%s\n", err)
			tracking.SendMetric("data.buildpr.error")
			return
		}
		ds.mutex.Lock()
		ds.allPRs[newPR.PR.GetNodeID()] = newPR
		ds.mutex.Unlock()
		ds.prUpdateChan <- newPR
	}

	ds.SaveToFile()
	ds.statusChan <- "" // clear status after each PR
}
