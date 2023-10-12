package datasource

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-github/v53/github"
	"github.com/inburst/prty/config"
	"github.com/inburst/prty/logger"
	"github.com/inburst/prty/tracking"
)

// // Suppressed errors will cause the cache file to be emptied and rebuilt
// // effectively self healing from corrupt or invalid data
// func (ds *Datasource) loadOrgsFile() []*github.Organization {
// 	orgCache := []*github.Organization{}

// 	cacheFilePath, err := config.GetOrgsCacheFilePath()
// 	if err != nil {
// 		tracking.SendMetric("data.loadsorgscache.patherror")
// 		return orgCache
// 	}
// 	data, err := ioutil.ReadFile(cacheFilePath)
// 	if err != nil {
// 		tracking.SendMetric("data.loadsorgscache.readerror")
// 		return orgCache
// 	}
// 	err = json.Unmarshal(data, &orgCache)
// 	if err != nil {
// 		tracking.SendMetric("data.loadsorgscache.unmarshallerror")
// 	}
// 	return orgCache
// }

// func (ds *Datasource) SaveOrgsFile() {
// 	cacheFilePath, _ := config.GetOrgsCacheFilePath()
// 	ds.orgsMutex.Lock()
// 	file, _ := json.MarshalIndent(ds.cachedOrgs, "", " ")
// 	ds.orgsMutex.Unlock()
// 	_ = ioutil.WriteFile(cacheFilePath, file, 0644)
// }

// func (ds *Datasource) getCachedOrg(org string) *github.Organization {
// 	if cachedOrg, ok := ds.cachedOrgs[fmt.Sprintf("%s", org)]; ok {
// 		return cachedOrg
// 	}
// 	return nil
// }

/**
#!/bin/bash
curl -L \
  -H "Accept: application/vnd.github+json" \
  -H "Authorization: Bearer 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" \
  -H "X-GitHub-Api-Version: 2022-11-28" \
  "https://api.github.com/issues?" | \
jq '
.[] |
select(
  has("pull_request")
) | .pull_request.html_url'

# ?since=2023-01-01T00:00:00Z

*/

func (ds *Datasource) getCachedIssues() map[string]*github.Issue {
	return ds.cachedIssues
}

// Suppressed errors will cause the cache file to be emptied and rebuilt
// effectively self healing from corrupt or invalid data
func (ds *Datasource) loadIssuesFile() map[string]*github.Issue {
	issuesCache := map[string]*github.Issue{}

	cacheFilePath, err := config.GetIssuesCacheFilePath()
	if err != nil {
		tracking.SendMetric("data.loadissuescache.patherror")
		return issuesCache
	}
	data, err := ioutil.ReadFile(cacheFilePath)
	if err != nil {
		tracking.SendMetric("data.loadissuescache.readerror")
		return issuesCache
	}
	err = json.Unmarshal(data, &issuesCache)
	if err != nil {
		tracking.SendMetric("data.loadissuescache.unmarshallerror")
	}
	return issuesCache
}

func (ds *Datasource) SaveIssuesFile() {
	cacheFilePath, _ := config.GetIssuesCacheFilePath()
	ds.issuesMutex.Lock()
	file, _ := json.MarshalIndent(ds.cachedIssues, "", " ")
	ds.issuesMutex.Unlock()
	_ = ioutil.WriteFile(cacheFilePath, file, 0644)
}

func (ds *Datasource) getMostRecentIssueUpdatedAt() time.Time {
	var mostRecentActivityTime time.Time
	for _, issue := range ds.cachedIssues {
		activityInfo := ds.activityInfoFromIssue(issue)
		if activityInfo != nil {
			if mostRecentActivityTime.IsZero() || activityInfo.updatedAt.After(mostRecentActivityTime) {
				mostRecentActivityTime = activityInfo.updatedAt
			}
		}
	}
	return mostRecentActivityTime
}

func (ds *Datasource) updateIssueCache(issues []*github.Issue) {
	if len(issues) == 0 {
		return
	}
	for _, issue := range issues {
		if issue.PullRequestLinks != nil {
			activityInfo := ds.activityInfoFromIssue(issue)
			if activityInfo != nil {
				ds.cachedIssues[activityInfo.cacheKey()] = issue
			}
		}
	}

	// filter any abandoned prs
	for key, issue := range ds.cachedIssues {
		if issue.GetUpdatedAt().Before(time.Now().AddDate(0, 0, ds.config.AbandonedAgeDays*-1)) {
			delete(ds.cachedIssues, key)
		}
	}

	ds.SaveIssuesFile()
}

func (ds *Datasource) GetUpdatedIssues(since time.Time) ([]*github.Issue, error) {
	logger.Shared().Printf("finding issues updated since %s\n", since)

	ctx := context.Background()

	opt := &github.IssueListOptions{
		Filter: "all",
		// NOTE: open issue filter calculation is either calculated hourly or is on
		// a noticeable delay. Activity in the last ~ 1 hr wont be picked up for
		// now. This is reasonable acceptable .....
		State: "open",
		// NOTE: sort and direction do not seem to be honored when state:open is used
		Sort: "updated",
		// Direction: "desc",
		Since: since,
		ListOptions: github.ListOptions{
			PerPage: GHResultsPerPage,
		},
	}

	// get all pages of results
	var allOrgs []*github.Issue
	for {
		issues, resp, err := sharedClient().Issues.List(ctx, true, opt)

		if _, ok := err.(*github.RateLimitError); ok {
			logger.Shared().Printf("issues: hit rate limit")
			untilReset := resp.Rate.Reset.Sub(time.Now())
			minUntilReset := untilReset.Round(time.Minute) / time.Minute
			ds.writeStatus(fmt.Sprintf("hit rate limit, waiting %02dm", minUntilReset))
			time.Sleep(untilReset)
			continue
		} else if arlerr, ok := err.(*github.AbuseRateLimitError); ok {
			logger.Shared().Printf("issues: hit secondary rate limit")
			time.Sleep(*arlerr.RetryAfter)
			continue
		} else if err != nil {
			logger.Shared().Printf("issues err: %s\n", err)
			return allOrgs, err
		}

		ds.remainingRequestsChan <- resp.Rate
		logger.Shared().Printf("found [%d] recently updated issues %d %s\n", len(issues), resp.NextPage, issues[len(issues)-1].GetUpdatedAt())
		allOrgs = append(allOrgs, issues...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	return allOrgs, nil
}

func (ds *Datasource) extractRepoActivityFromIssues(issues []*github.Issue) []orgAndRepo {
	uniqueMap := make(map[string]map[string]bool)

	for _, issue := range issues {
		if issue.PullRequestLinks != nil {
			if _, ok := uniqueMap[issue.GetRepository().GetOwner().GetLogin()]; !ok {
				uniqueMap[issue.GetRepository().GetOwner().GetLogin()] = make(map[string]bool)
			}
			uniqueMap[issue.GetRepository().GetOwner().GetLogin()][issue.GetRepository().GetName()] = true
		}
	}
	logger.Shared().Println("Detected activity on the following repos:")
	var results []orgAndRepo
	for orgName, repoNameList := range uniqueMap {
		for repoName, _ := range repoNameList {
			results = append(results, orgAndRepo{orgName, repoName})
			logger.Shared().Printf("%s/%s", orgName, repoName)
		}
	}

	return results
}

func (ds *Datasource) extractPullRequestActivityFromIssues() []*ghprActivity {
	var results = []*ghprActivity{}

	for _, issue := range ds.cachedIssues {
		if issue.PullRequestLinks != nil {
			activityInfo := ds.activityInfoFromIssue(issue)
			if activityInfo != nil {
				results = append(results, activityInfo)
			}
		}
	}
	return results
}

func (ds *Datasource) activityInfoFromIssue(issue *github.Issue) *ghprActivity {
	if issue == nil {
		return nil
	}
	prHTMLURL := issue.PullRequestLinks.GetHTMLURL()
	u, err := url.Parse(prHTMLURL)
	if err != nil {
		logger.Shared().Printf("error parsing pr url: %s %s\n", prHTMLURL, err)
		return nil
	}
	pathParts := strings.Split(u.Path, "/")
	if len(pathParts) < 5 {
		logger.Shared().Printf("error expected 5 path parts in url, got %d : %s\n", len(pathParts), prHTMLURL)
		return nil
	}
	orgName := pathParts[1]
	repoName := pathParts[2]
	prNumberStr := pathParts[4]
	prNumber, err := strconv.Atoi(prNumberStr)
	if err != nil {
		logger.Shared().Printf("error parsing pr number [%s] %s\n", prNumberStr, err)
		return nil
	}

	return &ghprActivity{
		orgName,
		repoName,
		prNumber,
		issue.GetUpdatedAt().Time,
	}
}

func (act *ghprActivity) cacheKey() string {
	return fmt.Sprintf("%s/%s/%d", act.orgName, act.repoName, act.prNumber)
}
