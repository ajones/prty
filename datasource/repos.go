package datasource

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/google/go-github/v53/github"
	"github.com/inburst/prty/config"
	"github.com/inburst/prty/logger"
	"github.com/inburst/prty/tracking"
)

// Suppressed errors will cause the cache file to be emptied and rebuilt
// effectively self healing from corrupt or invalid data
func (ds *Datasource) loadReposFile() map[string]map[string]*github.Repository {
	repoCache := map[string]map[string]*github.Repository{}

	cacheFilePath, err := config.GetReposCacheFilePath()
	if err != nil {
		tracking.SendMetric("data.loadreposcache.patherror")
		return repoCache
	}
	data, err := ioutil.ReadFile(cacheFilePath)
	if err != nil {
		tracking.SendMetric("data.loadreposcache.readerror")
		return repoCache
	}
	err = json.Unmarshal(data, &repoCache)
	if err != nil {
		tracking.SendMetric("data.loadreposcache.unmarshallerror")
	}
	return repoCache
}

func (ds *Datasource) SaveReposFile() {
	cacheFilePath, _ := config.GetReposCacheFilePath()
	ds.reposMutex.Lock()
	file, _ := json.MarshalIndent(ds.cachedRepos, "", " ")
	ds.reposMutex.Unlock()
	_ = ioutil.WriteFile(cacheFilePath, file, 0644)
}

func (ds *Datasource) getCachedReposForOrg(orgName string) []*github.Repository {
	if reposInOrg, ok := ds.cachedRepos[orgName]; ok {
		cachedRepos := []*github.Repository{}
		for _, repo := range reposInOrg {
			cachedRepos = append(cachedRepos, repo)
		}
		return cachedRepos
	}
	return nil
}

func (ds *Datasource) getCachedRepo(orgName, repoName string) *github.Repository {
	if reposInOrg, ok := ds.cachedRepos[orgName]; ok {
		if cachedRepo, ok := reposInOrg[repoName]; ok {
			return cachedRepo
		}
	}
	return nil
}

func (ds *Datasource) addRepoToCache(orgName, repoName string, repo *github.Repository) {
	if _, ok := ds.cachedRepos[orgName]; !ok {
		ds.cachedRepos[orgName] = map[string]*github.Repository{}
	}
	ds.cachedRepos[orgName][repoName] = repo
	ds.SaveReposFile()
}

func (ds *Datasource) GetAllReposForOrg(orgName string) ([]*github.Repository, error) {
	ctx := context.Background()

	opt := &github.RepositoryListByOrgOptions{
		ListOptions: github.ListOptions{PerPage: GHResultsPerPage},
		Sort:        "updated",
		Direction:   "desc",
		Type:        "all",
	}

	// get all pages of results
	var allRepos []*github.Repository
	for {
		repos, resp, err := sharedClient().Repositories.ListByOrg(ctx, orgName, opt)

		if _, ok := err.(*github.RateLimitError); ok {
			logger.Shared().Printf("repos: hit rate limit [%s]", orgName)
			untilReset := resp.Rate.Reset.Sub(time.Now())
			minUntilReset := untilReset.Round(time.Minute) / time.Minute
			ds.writeStatus(fmt.Sprintf("hit rate limit, waiting %02dm", minUntilReset))
			time.Sleep(untilReset)
			continue
		} else if arlerr, ok := err.(*github.AbuseRateLimitError); ok {
			logger.Shared().Printf("repos: hit secondary rate limit [%s]", orgName)
			time.Sleep(*arlerr.RetryAfter)
			continue
		} else if err != nil {
			logger.Shared().Printf("repos err for org [%s]: %s\n", orgName, err)
			return allRepos, err
		}

		ds.remainingRequestsChan <- resp.Rate
		logger.Shared().Printf("found [%d] repos in org [%s]\n", len(repos), orgName)
		allRepos = append(allRepos, repos...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	return allRepos, nil
}

func (ds *Datasource) GetRepoInOrg(orgName string, repoName string) (*github.Repository, error) {
	ctx := context.Background()
	repo, _, err := sharedClient().Repositories.Get(ctx, orgName, repoName)
	if err != nil {
		logger.Shared().Printf("repo err for [%s/%s]: %s\n", orgName, repoName, err)
		return nil, err
	}
	logger.Shared().Printf("found repos [%s/%s]\n", orgName, repoName)
	return repo, nil
}
