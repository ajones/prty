package datasource

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/go-github/v53/github"
	"github.com/inburst/prty/config"
	"github.com/inburst/prty/logger"
	"github.com/inburst/prty/tracking"
	"golang.org/x/exp/slices"
	"golang.org/x/oauth2"
)

const GHResultsPerPage = 100

type Datasource struct {
	statusChan            chan<- string
	prUpdateChan          chan<- *PullRequest
	remainingRequestsChan chan<- github.Rate

	config *config.Config

	cachedOrgs          map[string]*github.Organization
	cachedRepos         map[string]map[string]*github.Repository
	cachedPRs           map[string]*PullRequest
	cachedIssues        map[string]*github.Issue
	currentlyRefreshing bool

	repoEvents map[string][]*github.Event

	mutex       sync.RWMutex
	eventsMutex sync.RWMutex
	reposMutex  sync.RWMutex
	orgsMutex   sync.RWMutex
	issuesMutex sync.RWMutex
}

type orgAndRepo struct {
	orgName  string
	repoName string
}

type ghprActivity struct {
	orgName   string
	repoName  string
	prNumber  int
	updatedAt time.Time
}

type jobRefreshPR struct {
	org  string
	repo string
	pr   *PullRequest
}

type jobRefreshRepo struct {
	orgName    string
	repoName   string
	repository *github.Repository
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
	logger.Shared().Printf("loading local cache...\n")

	ds.cachedPRs = ds.loadSaveFile()
	ds.repoEvents = ds.loadEventsFile()
	ds.cachedRepos = ds.loadReposFile()
	ds.cachedOrgs = ds.loadOrgsFile()
	ds.cachedIssues = ds.loadIssuesFile()

	for _, pr := range ds.cachedPRs {
		if pr.IsOpen() {
			ds.prUpdateChan <- pr
		}
	}
}

func (ds *Datasource) IsCurrentlyRefreshingData() bool {
	return ds.currentlyRefreshing
}

func (ds *Datasource) handleError(err error, willContinue bool, metric string) {
	ds.writeErrorStatus(err)
	logger.Shared().Printf("%s\n", err)
	tracking.SendMetric(metric)
	ds.currentlyRefreshing = willContinue
}

func (ds *Datasource) RefreshData() {
	tracking.SendMetric("data.refresh")
	ds.currentlyRefreshing = true

	var wgOrgs sync.WaitGroup
	var wgEvents sync.WaitGroup
	var wgPreprocessPR sync.WaitGroup
	var wgRefreshPR sync.WaitGroup

	const numWorkers = 2
	orgJobs := make(chan *github.Organization)
	eventsJobs := make(chan jobRefreshRepo)
	prPreprocessJobs := make(chan jobRefreshRepo)
	prRefreshJobs := make(chan jobRefreshPR)

	// start workers
	for w := 0; w < numWorkers; w++ {
		wgOrgs.Add(1)
		go ds.processOrgWorker(w, orgJobs, &wgOrgs, eventsJobs)
	}
	for w := 0; w < numWorkers; w++ {
		wgEvents.Add(1)
		go ds.refreshEventsWorker(w, eventsJobs, &wgEvents, prPreprocessJobs)
	}
	for w := 0; w < numWorkers; w++ {
		wgPreprocessPR.Add(1)
		go ds.preprocessRepoPRsWorker(w, prPreprocessJobs, &wgPreprocessPR, prRefreshJobs)
	}
	for w := 0; w < numWorkers; w++ {
		wgRefreshPR.Add(1)
		go ds.buildPRWorker(w, prRefreshJobs, &wgRefreshPR)
	}

	lookBack := time.Now().AddDate(0, 0, -1*ds.config.AbandonedAgeDays)
	lastIssueUpdatedAt := ds.getMostRecentIssueUpdatedAt()
	if lastIssueUpdatedAt.After(lookBack) {
		lookBack = lastIssueUpdatedAt
	}

	updatedIssues, err := ds.GetUpdatedIssues(lookBack)
	if err != nil {
		ds.writeErrorStatus(err)
		logger.Shared().Printf("%s\n", err)
		tracking.SendMetric("data.getissues.error")
		ds.currentlyRefreshing = false
		return
	}
	ds.updateIssueCache(updatedIssues)

	prsWithActivity := ds.extractPullRequestActivityFromIssues()
	logger.Shared().Printf("found [%d] PRs with activity\n", len(prsWithActivity))

	for _, act := range prsWithActivity {
		cachedPR := ds.getCachedPR(act.orgName, act.repoName, act.prNumber)

		if act.updatedAt.After(cachedPR.GithubPR.GetUpdatedAt().Time) {
			// update PR
			prRefreshJobs <- jobRefreshPR{
				org:  act.orgName,
				repo: act.repoName,
				pr:   cachedPR,
			}
		}
		if cachedPR.IsOpen() {
			// always re-calculated to allow time based signals
			// to change the importance of a PR
			cachedPR.calculateImportance(ds)

			ds.prUpdateChan <- cachedPR
		} else {
			// todo: move to fn, this is sloppy
			delete(ds.cachedIssues, act.cacheKey())
		}
	}

	// reposWithActivity := ds.extractRepoActivityFromIssues(updatedIssues)
	// for _, repoInfo := range reposWithActivity {
	// 	if len(ds.config.RepoWhitelist) != 0 && !listContains(ds.config.RepoWhitelist, repoInfo.repoName) {
	// 		logger.Shared().Printf("whitelist found and repo [%s] not included, skipping...\n", repoInfo.repoName)
	// 		continue
	// 	}
	// 	if len(ds.config.RepoBlacklist) != 0 && listContains(ds.config.RepoBlacklist, repoInfo.repoName) {
	// 		logger.Shared().Printf("blacklist found and repo [%s] not included, skipping...\n", repoInfo.repoName)
	// 		continue
	// 	}
	// 	ghRepo := ds.getCachedRepo(repoInfo.orgName, repoInfo.repoName)
	// 	if ghRepo == nil {
	// 		repoData, err := ds.GetRepoInOrg(repoInfo.orgName, repoInfo.repoName)
	// 		if err != nil {
	// 			ds.writeErrorStatus(err)
	// 			logger.Shared().Printf("%s\n", err)
	// 			tracking.SendMetric("data.getrepo.error")
	// 			continue
	// 		}
	// 		ghRepo = repoData
	// 		ds.addRepoToCache(repoInfo.orgName, repoInfo.repoName, repoData)
	// 	}
	// 	eventsJobs <- jobRefreshRepo{
	// 		orgName:    repoInfo.orgName,
	// 		repoName:   repoInfo.repoName,
	// 		repository: ghRepo,
	// 	}
	// }

	// // Determine all orgs to go check
	// ds.writeStatus("fetching users orgs...")
	// logger.Shared().Printf("loading all orgs for user...\n")
	// orgs, err := ds.GetAllOrgs()
	// if err != nil {
	// 	ds.writeErrorStatus(err)
	// 	logger.Shared().Printf("%s\n", err)
	// 	tracking.SendMetric("data.getorgs.error")
	// 	ds.currentlyRefreshing = false
	// 	return
	// }

	// for _, org := range orgs {
	// 	orgJobs <- org
	// }

	// close the jobs channel so the workers know to stop
	// close(orgJobs)
	// wait for all jobs to complete
	// wgOrgs.Wait()

	close(eventsJobs)
	wgEvents.Wait()

	close(prPreprocessJobs)
	wgPreprocessPR.Wait()

	close(prRefreshJobs)
	wgRefreshPR.Wait()

	ds.currentlyRefreshing = false
	ds.writeStatus("refreshed")
}

func (ds *Datasource) processOrgWorker(id int, jobs <-chan *github.Organization, wg *sync.WaitGroup, refreshRepoJobs chan<- jobRefreshRepo) {
	for j := range jobs {
		// logger.Shared().Printf("worker [%d/pr/%s/%s/%d] starting...", id, j.org, j.repo, j.pr.GetNumber())
		orgName := *j.Login

		if listContains(ds.config.OrgBlacklist, orgName) {
			logger.Shared().Printf("skipping due to blacklist %s\n", orgName)
			continue
		}
		if len(ds.config.OrgWhitelist) != 0 && !listContains(ds.config.OrgWhitelist, orgName) {
			logger.Shared().Printf("whitelist found and [%s] not included, skipping...\n", orgName)
			continue
		}

		cachedOrg := ds.getCachedOrg(orgName)
		var repos []*github.Repository = nil
		var err error = nil

		// NOTE: This is a working assumption...
		// updated_at will change when total_private_repos or owned_private_repos
		// changes signaling that we should re-pull the list of repos for this org
		if cachedOrg != nil ||
			!j.GetUpdatedAt().Time.After(cachedOrg.GetUpdatedAt().Time) {
			// Attempt to load repos from cache
			repos = ds.getCachedReposForOrg(orgName)
		}

		// we failed to load repos from cache so go het them
		if repos == nil {
			logger.Shared().Printf("loading all repos for org [%s]...\n", orgName)
			repos, err = ds.GetAllReposForOrg(orgName)
			if err != nil {
				ds.handleError(err, true, "data.getrepos.error")
				continue
			}
		}

		logger.Shared().Printf("loading all repos for org [%s]...\n", orgName)
		for _, repo := range repos {
			repoName := *repo.Name
			if len(ds.config.RepoWhitelist) != 0 && !listContains(ds.config.RepoWhitelist, repoName) {
				logger.Shared().Printf("whitelist found and repo [%s] not included, skipping...\n", repoName)
				continue
			}
			if len(ds.config.RepoBlacklist) != 0 && listContains(ds.config.RepoBlacklist, repoName) {
				logger.Shared().Printf("blacklist found and repo [%s] not included, skipping...\n", repoName)
				continue
			}

			// TODO: figure out hot to avoid refreshing all repos...
			refreshRepoJobs <- jobRefreshRepo{
				orgName:    orgName,
				repoName:   repoName,
				repository: repo,
			}

			// cachedRepo := ds.getCachedRepo(orgName, repoName)
			// // first time seeing repo
			// if cachedRepo == nil {
			// 	logger.Shared().Printf("repo %s/%s not found in cache, will refresh...\n", orgName, repoName)
			// 	refreshRepoJobs <- jobRefreshRepo{
			// 		orgName:    orgName,
			// 		repoName:   repoName,
			// 		repository: repo,
			// 	}
			// 	continue
			// }

			// // new commits to the repo
			// if !cachedRepo.GetPushedAt().Equal(repo.GetPushedAt()) {
			// 	// logger.Shared().Printf("no new activity for %s/%s skipping...\n", orgName, repoName)
			// 	logger.Shared().Printf("new commit activity for %s/%s will refresh...\n", orgName, repoName)
			// 	refreshRepoJobs <- jobRefreshRepo{
			// 		orgName:    orgName,
			// 		repoName:   repoName,
			// 		repository: repo,
			// 	}
			// 	continue
			// }

			// // New or closed issues
			// if cachedRepo.GetOpenIssuesCount() != repo.GetOpenIssuesCount() {
			// 	// logger.Shared().Printf("no new activity for %s/%s skipping...\n", orgName, repoName)
			// 	logger.Shared().Printf("new issue activity for %s/%s will refresh...\n", orgName, repoName)
			// 	refreshRepoJobs <- jobRefreshRepo{
			// 		orgName:    orgName,
			// 		repoName:   repoName,
			// 		repository: repo,
			// 	}
			// 	continue
			// }

			// logger.Shared().Printf("fall through to default refresh for %s/%s will refresh...\n", orgName, repoName)
			// refreshRepoJobs <- jobRefreshRepo{
			// 	orgName:    orgName,
			// 	repoName:   repoName,
			// 	repository: repo,
			// }
		}

		// logger.Shared().Printf("worker [%d/pr/%s/%s/%d] done", id, j.org, j.repo, j.pr.GetNumber())
	}
	wg.Done()
	// after we process all the orgs be sure to save them to the cache
	ds.SaveOrgsFile()
}

func (ds *Datasource) refreshEventsWorker(id int, jobs <-chan jobRefreshRepo, wg *sync.WaitGroup, preprocessRepoPRsJobs chan<- jobRefreshRepo) {
	// For each repo, identify PRs by recent activity
	for j := range jobs {
		var mostRecentEvent *time.Time = nil
		existingEvents := ds.getCachedEvents(j.orgName, j.repoName)
		if len(existingEvents) > 0 {
			createdAt := existingEvents[0].GetCreatedAt()
			mostRecentEvent = (&createdAt).GetTime()
		}

		ds.writeStatus(fmt.Sprintf("fetching events for %s/%s ...", j.orgName, j.repoName))
		events, err := ds.GetEventsForRepo(j.orgName, j.repoName, mostRecentEvent, ds.config.AbandonedAgeDays)
		if err != nil {
			ds.handleError(err, true, "data.getevents.error")
			continue
		}

		ds.addNewEvents(j.orgName, j.repoName, events)

		// ensure we cache the repo AFTER we refresh the events so that we can
		// compare the previous and new timestamps and avoid unnecessary refresh
		ds.addRepoToCache(j.orgName, j.repoName, j.repository)

		preprocessRepoPRsJobs <- j
	}
	wg.Done()
}

func (ds *Datasource) preprocessRepoPRsWorker(id int, jobs <-chan jobRefreshRepo, wg *sync.WaitGroup, refreshPRJobs chan<- jobRefreshPR) {
	// For each repo, identify PRs by recent activity
	for j := range jobs {
		cachedPRs, prsToLoad, miscUpdates := ds.getCachedAndRefreshPRListFromEvents(j.orgName, j.repoName)

		for _, pr := range cachedPRs {
			if slices.Contains(miscUpdates, pr.GithubPR.GetNumber()) {
				logger.Shared().Printf("misc activity detected for [%s/%s/%d]\n", j.orgName, j.repoName, pr.GithubPR.GetNumber())
				refreshPRJobs <- jobRefreshPR{
					org:  j.orgName,
					repo: j.repoName,
					pr:   pr,
				}
				continue
			}
			logger.Shared().Printf("no new activity for [%s/%s/%d]\n", j.orgName, j.repoName, pr.GithubPR.GetNumber())
			if pr.IsOpen() {
				ds.prUpdateChan <- pr
			}
			continue
		}

		logger.Shared().Printf("will load [%d] PRs for [%s/%s] \n", len(prsToLoad), j.orgName, j.repoName)
		for _, pr := range prsToLoad {
			// there is some activity on this PR. go update it
			refreshPRJobs <- jobRefreshPR{
				org:  j.orgName,
				repo: j.repoName,
				pr:   pr,
			}
		}
	}
	wg.Done()
}

func (ds *Datasource) buildPRWorker(id int, jobs <-chan jobRefreshPR, wg *sync.WaitGroup) {
	for j := range jobs {
		logger.Shared().Printf("worker [%d/pr/%s/%s/%d] starting...", id, j.org, j.repo, j.pr.Number)
		ds.conditionallyRefreshPRFromShallowGithubPR(j.org, j.repo, j.pr)
		logger.Shared().Printf("worker [%d/pr/%s/%s/%d] done", id, j.org, j.repo, j.pr.Number)
	}
	wg.Done()
}

func (ds *Datasource) conditionallyRefreshPRFromShallowGithubPR(orgName string, repoName string, pr *PullRequest) {
	if pr == nil {
		return
	}

	prData := ds.getCachedPR(orgName, repoName, pr.Number)
	ds.RefreshPR(prData)
	if prData.IsOpen() {
		ds.prUpdateChan <- prData
	}
	ds.statusChan <- "" // clear status after each PR
}
