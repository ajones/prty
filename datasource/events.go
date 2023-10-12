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

func (ds *Datasource) getCachedAndRefreshPRListFromEvents(org, repo string) ([]*PullRequest, []*PullRequest, []int) {
	events := ds.getCachedEvents(org, repo)

	cachedPRs := map[string]*PullRequest{}
	prsToBeUpdated := map[string]*PullRequest{}
	miscActivity := map[string]int{}

	for i := len(events) - 1; i >= 0; i-- {
		e := events[i]
		body, err := e.ParsePayload()
		if err != nil {
			ds.writeErrorStatus(err)
			logger.Shared().Printf("%s\n", err)
			tracking.SendMetric("data.eventparse.error")
			ds.currentlyRefreshing = false
			continue
		}

		// logger.Shared().Printf("~~~EVENT %s/%s -- %s\n", org, repo, e.GetRawPayload())

		var ghpr *github.PullRequest = nil
		// closedOrMerged := false
		if e.GetType() == "PullRequestEvent" {
			bodyDetails := body.(*github.PullRequestEvent)
			ghpr = bodyDetails.PullRequest
			if bodyDetails.GetAction() == "closed" {
				// delete(uniqiueMap, ghpr.GetNodeID())
				continue
			}

			cachedPR := ds.getCachedPR(org, repo, ghpr.GetNumber())
			// if we have not seen this PR before then always load it
			if cachedPR.GithubPR == nil {
				prsToBeUpdated[ghpr.GetNodeID()] = cachedPR
				continue
			}

			if cachedPR.LastUpdatedAt.After(e.GetCreatedAt().Time) {
				// this event is older than our last data update
				cachedPRs[cachedPR.GithubPR.GetNodeID()] = cachedPR
				// ensure it is not in the list to be updated
				delete(prsToBeUpdated, ghpr.GetNodeID())
				continue
			}
			// this PR will need to be updated
			prsToBeUpdated[ghpr.GetNodeID()] = cachedPR
		} else if e.GetType() == "PullRequestReviewEvent" {
			bodyDetails := body.(*github.PullRequestReviewEvent)
			ghpr = bodyDetails.PullRequest

			cachedPR := ds.getCachedPR(org, repo, ghpr.GetNumber())
			if cachedPR.GithubPR == nil {
				prsToBeUpdated[ghpr.GetNodeID()] = cachedPR
				continue
			}

			if cachedPR.LastUpdatedAt.After(e.GetCreatedAt().Time) {
				// no new activity
				cachedPRs[cachedPR.GithubPR.GetNodeID()] = cachedPR
				// ensure it is not in the list to be updated
				delete(prsToBeUpdated, ghpr.GetNodeID())
				continue
			}
			// this PR will need to be updated
			prsToBeUpdated[ghpr.GetNodeID()] = cachedPR
		} else if e.GetType() == "PullRequestReviewCommentEvent" {
			bodyDetails := body.(*github.PullRequestReviewCommentEvent)
			ghpr = bodyDetails.PullRequest

			cachedPR := ds.getCachedPR(org, repo, ghpr.GetNumber())
			if cachedPR.GithubPR == nil {
				prsToBeUpdated[ghpr.GetNodeID()] = cachedPR
				continue
			}

			if cachedPR.LastUpdatedAt.After(e.GetCreatedAt().Time) {
				// no new activity
				cachedPRs[cachedPR.GithubPR.GetNodeID()] = cachedPR
				// ensure it is not in the list to be updated
				delete(prsToBeUpdated, ghpr.GetNodeID())
				continue
			}
			// this PR will need to be updated
			prsToBeUpdated[ghpr.GetNodeID()] = cachedPR
		} else if e.GetType() == "IssueCommentEvent" {
			bodyDetails := body.(*github.IssueCommentEvent)
			issue := bodyDetails.Issue

			cachedPR := ds.getCachedPR(org, repo, issue.GetNumber())
			if cachedPR.LastUpdatedAt.After(e.GetCreatedAt().Time) {
				// ignore this update because our data is newer
				continue
			}

			// issue.number corresponds to the PR number
			// issue.closed_at will be null if the PR is open
			if issue.ClosedAt != nil {
				delete(cachedPRs, issue.GetNodeID())
				delete(prsToBeUpdated, issue.GetNodeID())
			} else {
				// PR is still open and there is a new comment
				// we want to capture this for importance checking
				miscActivity[issue.GetNodeID()] = issue.GetNumber()
			}
		} else if e.GetType() == "IssueEvent" {
			bodyDetails := body.(*github.IssueEvent)
			issue := bodyDetails.Issue

			cachedPR := ds.getCachedPR(org, repo, issue.GetNumber())
			if cachedPR.LastUpdatedAt.After(e.GetCreatedAt().Time) {
				// ignore this update because our data is newer
				continue
			}
			// issue.number corresponds to the PR number
			// issue.closed_at will be null if the PR is open
			if issue.ClosedAt != nil {
				delete(cachedPRs, issue.GetNodeID())
				delete(prsToBeUpdated, issue.GetNodeID())
				delete(miscActivity, issue.GetNodeID())

			} else {
				// PR is still open and there is a new comment
				// we want to capture this for importance checking
				miscActivity[issue.GetNodeID()] = issue.GetNumber()
			}
		}
	}

	// convert map to array
	fullToUpdatePRList := []*PullRequest{}
	for _, pr := range prsToBeUpdated {
		if pr != nil {
			fullToUpdatePRList = append(fullToUpdatePRList, pr)
		}
	}

	fullCachedPRList := []*PullRequest{}
	for _, pr := range cachedPRs {
		if pr != nil {
			fullCachedPRList = append(fullCachedPRList, pr)
		}
	}

	allMiscUpdates := []int{}
	for _, prNumber := range miscActivity {
		if prNumber > 0 {
			allMiscUpdates = append(allMiscUpdates, prNumber)
		}
	}

	return fullCachedPRList, fullToUpdatePRList, allMiscUpdates
}

func (ds *Datasource) addNewEvents(org, repo string, events []*github.Event) {
	// todo: use a channel not a mutex
	ds.eventsMutex.Lock()
	defer ds.eventsMutex.Unlock()

	if existingEvents, ok := ds.repoEvents[fmt.Sprintf("%s/%s", org, repo)]; ok && len(existingEvents) > 0 {
		toAppend := []*github.Event{}

		// last event time
		lastCachedEvent := existingEvents[0]
		lastCachedEventTime := lastCachedEvent.GetCreatedAt()
		let := *lastCachedEventTime.GetTime()

		for _, event := range events {
			// time of this event
			et := event.GetCreatedAt()
			if et.GetTime().After(let) {
				toAppend = append(toAppend, event)
			}
		}
		logger.Shared().Printf("discovered [%d] new events for %s/%s\n", len(toAppend), org, repo)

		// Events are returned from the api as most recent first. We want to
		// preserve this order so we will prepend the new events
		ds.setEvents(org, repo, append(toAppend, existingEvents...))
	} else {
		ds.setEvents(org, repo, events)
	}

	ds.SaveEventsFile()
}

func (ds *Datasource) setEvents(org, repo string, events []*github.Event) {
	ds.repoEvents[fmt.Sprintf("%s/%s", org, repo)] = trimEvents(events)
}

// only keep the last 30 days of events
func trimEvents(events []*github.Event) []*github.Event {
	inWindow := []*github.Event{}
	for _, event := range events {
		if event.GetCreatedAt().After(time.Now().AddDate(0, -1, 0)) {
			inWindow = append(inWindow, event)
		}
	}
	return inWindow
}

func (ds *Datasource) getCachedEvents(org, repo string) []*github.Event {
	// todo: move to map[string]map[string]... for more flexable cache processing
	if existingEvents, ok := ds.repoEvents[fmt.Sprintf("%s/%s", org, repo)]; ok {
		return existingEvents
	}
	return []*github.Event{}
}

// Suppressed errors will cause the cache file to be emptied and rebuilt
// effectively self healing from corrupt or invalid data
func (ds *Datasource) loadEventsFile() map[string][]*github.Event {
	eventCache := map[string][]*github.Event{}
	cacheFilePath, err := config.GetEventsCacheFilePath()
	if err != nil {
		tracking.SendMetric("data.loadeventscache.patherror")
		return eventCache
	}
	data, err := ioutil.ReadFile(cacheFilePath)
	if err != nil {
		tracking.SendMetric("data.loadeventscache.readerror")
		return eventCache
	}
	err = json.Unmarshal(data, &eventCache)
	if err != nil {
		tracking.SendMetric("data.loadeventscache.unmarshallerror")
	}
	return eventCache
}

func (ds *Datasource) SaveEventsFile() {
	cacheFilePath, _ := config.GetEventsCacheFilePath()
	ds.mutex.Lock()
	file, _ := json.MarshalIndent(ds.repoEvents, "", " ")
	ds.mutex.Unlock()
	_ = ioutil.WriteFile(cacheFilePath, file, 0644)
}

func (ds *Datasource) GetEventsForRepo(org string, repo string, lastEvent *time.Time, maxLookBackDays int) ([]*github.Event, error) {
	ctx := context.Background()

	opts := &github.ListOptions{PerPage: GHResultsPerPage}

	// get all pages of results
	var allEvents []*github.Event
	for {
		eventList, resp, err := sharedClient().Activity.ListRepositoryEvents(ctx, org, repo, opts)

		if _, ok := err.(*github.RateLimitError); ok {
			logger.Shared().Printf("events: hit rate limit [%s/%s]", org, repo)
			untilReset := resp.Rate.Reset.Sub(time.Now())
			minUntilReset := untilReset.Round(time.Minute) / time.Minute
			ds.writeStatus(fmt.Sprintf("hit rate limit, waiting %02dm", minUntilReset))
			time.Sleep(untilReset)
			continue
		} else if arlerr, ok := err.(*github.AbuseRateLimitError); ok {
			logger.Shared().Printf("events: hit secondary rate limit [%s]", org)
			time.Sleep(*arlerr.RetryAfter)
			continue
		} else if err != nil {
			logger.Shared().Printf("events api err for [%s/%s]: %s\n", org, repo, err)
			return allEvents, err
		}
		ds.remainingRequestsChan <- resp.Rate

		logger.Shared().Printf("pulled [%d] events from api for [%s/%s]\n", len(eventList), org, repo)
		allEvents = append(allEvents, eventList...)
		if resp.NextPage == 0 {
			logger.Shared().Printf("done pulling events from api, reached last page for [%s/%s]\n", org, repo)
			break
		}
		opts.Page = resp.NextPage

		if lastEvent != nil && eventList[len(eventList)-1].GetCreatedAt().Before(*lastEvent) {
			// Pull events back to the last one we have in the cache
			logger.Shared().Printf("done pulling events from api, reached top of cache for [%s/%s]\n", org, repo)
			break
		} else if eventList[len(eventList)-1].GetCreatedAt().Before(time.Now().AddDate(0, 0, maxLookBackDays*-1)) {
			// at max only pull up to 1 month back
			logger.Shared().Printf("done pulling events from api, reached [%d] look back days for [%s/%s]\n", maxLookBackDays, org, repo)
			break
		}
	}

	return allEvents, nil
}
