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
func (ds *Datasource) loadOrgsFile() map[string]*github.Organization {
	orgCache := make(map[string]*github.Organization)

	cacheFilePath, err := config.GetOrgsCacheFilePath()
	if err != nil {
		tracking.SendMetric("data.loadsorgscache.patherror")
		return orgCache
	}
	data, err := ioutil.ReadFile(cacheFilePath)
	if err != nil {
		tracking.SendMetric("data.loadsorgscache.readerror")
		return orgCache
	}
	err = json.Unmarshal(data, &orgCache)
	if err != nil {
		tracking.SendMetric("data.loadsorgscache.unmarshallerror")
	}
	return orgCache
}

func (ds *Datasource) SaveOrgsFile() {
	cacheFilePath, _ := config.GetOrgsCacheFilePath()
	ds.orgsMutex.Lock()
	file, _ := json.MarshalIndent(ds.cachedOrgs, "", " ")
	ds.orgsMutex.Unlock()
	_ = ioutil.WriteFile(cacheFilePath, file, 0644)
}

func (ds *Datasource) getCachedOrg(org string) *github.Organization {
	if cachedOrg, ok := ds.cachedOrgs[fmt.Sprintf("%s", org)]; ok {
		return cachedOrg
	}
	return nil
}

func (ds *Datasource) GetAllOrgs() ([]*github.Organization, error) {
	ctx := context.Background()
	opt := &github.ListOptions{PerPage: GHResultsPerPage}
	// get all pages of results
	var allOrgs []*github.Organization
	for {
		orgs, resp, err := sharedClient().Organizations.List(ctx, "", opt)

		if _, ok := err.(*github.RateLimitError); ok {
			logger.Shared().Printf("orgs: hit rate limit")
			untilReset := resp.Rate.Reset.Sub(time.Now())
			minUntilReset := untilReset.Round(time.Minute) / time.Minute
			ds.writeStatus(fmt.Sprintf("hit rate limit, waiting %02dm", minUntilReset))
			time.Sleep(untilReset)
			continue
		} else if arlerr, ok := err.(*github.AbuseRateLimitError); ok {
			logger.Shared().Printf("orgs: hit secondary rate limit")
			time.Sleep(*arlerr.RetryAfter)
			continue
		} else if err != nil {
			logger.Shared().Printf("orgs err: %s\n", err)
			return allOrgs, err
		}

		ds.remainingRequestsChan <- resp.Rate
		logger.Shared().Printf("found [%d] orgs\n", len(orgs))
		allOrgs = append(allOrgs, orgs...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	return allOrgs, nil
}

func (ds *Datasource) GetOrg(orgName string) (*github.Organization, error) {
	ctx := context.Background()

	org, _, err := sharedClient().Organizations.Get(ctx, orgName)
	if err != nil {
		logger.Shared().Printf("public org err: %s\n", err)
		return nil, err
	}
	return org, nil
}
