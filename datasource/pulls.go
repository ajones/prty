package datasource

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"time"

	"github.com/google/go-github/v53/github"
	"github.com/inburst/prty/config"
	"github.com/inburst/prty/logger"
	"github.com/inburst/prty/tracking"
)

type PullRequest struct {
	RepoName string
	OrgName  string
	Number   int

	LastUpdatedAt time.Time

	GithubPR         *github.PullRequest
	Commits          []*github.RepositoryCommit
	Comments         []*github.PullRequestComment
	Reviews          []*github.PullRequestReview
	Status           *github.CombinedStatus
	LastCommitsPage  int
	LastCommentsPage int
	LastReviewsPage  int

	Author string

	Labels             []string
	RequestedReviewers []string
	NumCommits         int

	FirstCommitTime time.Time
	LastCommitTime  time.Time

	LastCommentTime       time.Time
	LastCommentFromMeTime *time.Time

	IAmAuthor                  bool
	AuthorIsTeammate           bool
	AuthorIsBot                bool
	HasChangesAfterLastComment bool
	HasCommentsFromMe          bool
	LastCommentFromMe          bool
	TimeSinceLastCommentFromMe *time.Duration
	TimeSinceLastComment       time.Duration
	TimeSinceLastCommit        time.Duration
	TimeSinceFirstCommit       time.Duration
	IsApproved                 bool
	IsAbandoned                bool
	IsDraft                    bool
	Additions                  int
	Deletions                  int
	CodeDelta                  int
	JobStateSuccess            bool
	Mergeable                  bool
	HasNewChanges              bool

	Importance       float64
	ImportanceLookup map[string]float64

	ViewedAt *time.Time
}

func (pr *PullRequest) IsOpen() bool {
	if pr.GithubPR != nil {
		return pr.GithubPR.GetState() == "open"
	}
	return false
}

// returns the cached data or a new PullRequest if one does not exist
func (ds *Datasource) cachePR(pr *PullRequest) {
	ds.writeStatus(fmt.Sprintf("%s/%s/%d caching pr", pr.OrgName, pr.RepoName, pr.Number))
	ds.mutex.Lock()
	ds.cachedPRs[fmt.Sprintf("%s/%s/%d", pr.OrgName, pr.RepoName, pr.Number)] = pr
	ds.mutex.Unlock()
}

// returns the cached data or a new PullRequest if one does not exist
func (ds *Datasource) getCachedPR(org, repo string, prNumber int) *PullRequest {
	if cachedPR, ok := ds.cachedPRs[fmt.Sprintf("%s/%s/%d", org, repo, prNumber)]; ok {
		return cachedPR
	}
	return &PullRequest{
		OrgName:  org,
		RepoName: repo,
		Number:   prNumber,
	}
}

// Suppressed errors will cause the cache file to be emptied and rebuilt
// effectively self healing from corrupt or invalid data
func (ds *Datasource) loadSaveFile() map[string]*PullRequest {
	prs := map[string]*PullRequest{}
	cacheFilePath, err := config.GetPRCacheFilePath()
	if err != nil {
		tracking.SendMetric("data.loadprcache.patherror")
		return prs
	}
	data, err := ioutil.ReadFile(cacheFilePath)
	if err != nil {
		tracking.SendMetric("data.loadprcache.readerror")
		return prs
	}
	err = json.Unmarshal(data, &prs)
	if err != nil {
		tracking.SendMetric("data.loadprcache.unmarshallerror")
	}

	ds.trimPRCache()
	return prs
}

func (ds *Datasource) trimPRCache() {
	stillRelevantPRs := map[string]*PullRequest{}
	for k, pr := range ds.cachedPRs {
		if pr.GithubPR.GetUpdatedAt().After(time.Now().AddDate(0, 0, ds.config.AbandonedAgeDays*-1)) {
			stillRelevantPRs[k] = pr
		}
	}
	ds.cachedPRs = stillRelevantPRs
}

func (ds *Datasource) SavePRCacheToFile() {
	cacheFilePath, _ := config.GetPRCacheFilePath()
	ds.mutex.Lock()
	fileData, _ := json.MarshalIndent(ds.cachedPRs, "", " ")
	ds.mutex.Unlock()

	// ds.writeStatus(fmt.Sprintf("saving pr cache to [%s]...", cacheFilePath))
	err := ioutil.WriteFile(cacheFilePath, fileData, 0644)
	if err != nil {
		logger.Shared().Printf("error writing pr cache file %s", err)
	}
}

func (pr *PullRequest) calculateStatusFields(orgName string, repoName string, ds *Datasource) {
	pr.Author = pr.GithubPR.User.GetLogin()
	pr.OrgName = orgName
	pr.RepoName = repoName
	pr.NumCommits = len(pr.Commits)
	pr.IsDraft = pr.GithubPR.GetDraft()
	pr.IAmAuthor = (pr.Author == ds.config.GithubUsername)
	pr.Additions = pr.GithubPR.GetAdditions()
	pr.Deletions = pr.GithubPR.GetDeletions()
	pr.CodeDelta = pr.Additions + pr.Deletions
	pr.JobStateSuccess = pr.Status.GetState() == "success"
	pr.Mergeable = pr.GithubPR.GetMergeable()

	// todo: write a util fn
	for _, n := range ds.config.TeamUsernames {
		if n == pr.Author {
			pr.AuthorIsTeammate = true
			break
		}
	}

	// todo: write a util fn
	for _, n := range ds.config.BotUsernames {
		if n == pr.Author {
			pr.AuthorIsBot = true
			break
		}
	}

	// Comments
	// start the comment time at the PR creation time
	lastCommentTime := *pr.GithubPR.CreatedAt.GetTime()
	var lastCommentFromMeTime *time.Time = nil
	for _, c := range pr.Comments {
		if lastCommentTime.Before(*c.CreatedAt.GetTime()) {
			lastCommentTime = *c.CreatedAt.GetTime()
		}

		// mark if I have commented on this PR
		if c.User.GetLogin() == ds.config.GithubUsername {
			pr.HasCommentsFromMe = true

			if lastCommentFromMeTime == nil || lastCommentFromMeTime.Before(*c.CreatedAt.GetTime()) {
				lastCommentFromMeTime = c.CreatedAt.GetTime()
			}
		}
	}
	pr.LastCommentTime = lastCommentTime
	pr.TimeSinceLastComment = time.Since(lastCommentTime)
	if lastCommentFromMeTime != nil {
		pr.LastCommentFromMeTime = lastCommentFromMeTime
		dur := time.Since(*lastCommentFromMeTime)
		pr.TimeSinceLastCommentFromMe = &dur
	}

	// Commits
	firstCommitTime := *pr.GithubPR.CreatedAt.GetTime()
	lastCommitTime := *pr.GithubPR.CreatedAt.GetTime()
	for _, c := range pr.Commits {
		if firstCommitTime.After(*c.Commit.Committer.Date.GetTime()) {
			firstCommitTime = *c.Commit.Committer.Date.GetTime()
		}
		if lastCommitTime.Before(*c.Commit.Committer.Date.GetTime()) {
			lastCommitTime = *c.Commit.Committer.Date.GetTime()
		}
	}
	pr.FirstCommitTime = firstCommitTime
	pr.LastCommitTime = lastCommitTime
	pr.TimeSinceLastCommit = time.Since(lastCommitTime)
	pr.TimeSinceFirstCommit = time.Since(firstCommitTime)

	// Are there changes after my last comment
	pr.HasNewChanges = pr.LastCommentFromMeTime != nil && pr.LastCommitTime.After(*pr.LastCommentFromMeTime)

	// Abandoned
	abandonThresholdTime := lastCommitTime.Add(time.Duration(21) * time.Hour * time.Duration(24))
	pr.IsAbandoned = time.Now().After(abandonThresholdTime)

	/*
		// TODO : PULL THIS OUT TO NOT BE READ EVERY TIME!!!
		abandonAgeDays := os.Getenv("ABANDONED_AGE_DAYS")
		if days, err := strconv.Atoi(abandonAgeDays); err == nil {
			pr.IsAbandoned = time.Now().After(pr.LastCommitTime.Add(time.Duration(21) * time.Hour * time.Duration(24)))
		}
	*/

	// flag if there are changes after last comment
	if len(pr.Comments) == 0 || lastCommentTime.Before(lastCommitTime) {
		pr.HasChangesAfterLastComment = true
	}

	for i := range pr.GithubPR.Labels {
		l := pr.GithubPR.Labels[i]
		pr.Labels = append(pr.Labels, *l.Name)
	}

	for i := range pr.GithubPR.RequestedReviewers {
		r := pr.GithubPR.RequestedReviewers[i]
		pr.RequestedReviewers = append(pr.RequestedReviewers, *r.Login)
	}

	for _, r := range pr.Reviews {
		if *r.State == "APPROVED" {
			pr.IsApproved = true
		}
	}

	// clear viewed at if there are new changes
	if pr.ViewedAt != nil {
		if lastCommitTime.After(*pr.ViewedAt) || lastCommentTime.After(*pr.ViewedAt) {
			pr.ViewedAt = nil
		}
	}
}

// All importance values are normilized to 0-100
func (pr *PullRequest) calculateImportance(ds *Datasource) {
	// ensure all status fields are precalculated
	pr.calculateStatusFields(pr.OrgName, pr.RepoName, ds)

	pr.ImportanceLookup = make(map[string]float64)
	importance := 0.0

	// TODOs:
	// add importance if i am CODEOWNER
	// recent activity is more important ?? maybe??
	// if the job state is not "success" it is less important

	// if this pr is abandoned then we don't need to look at it
	if pr.IsAbandoned {
		pr.ImportanceLookup["Abandoned"] = 0
		return
	}

	// if I am not the author and it is approved we don't need to look at it
	if !pr.IAmAuthor && pr.IsApproved {
		pr.ImportanceLookup["Approved"] = 0
		return
	}

	// if it is mine and approved we should go look at it
	if pr.IAmAuthor && pr.IsApproved {
		importance = math.MaxFloat64
		pr.ImportanceLookup["Ready"] = math.MaxFloat64
		return
	}

	// if this pr is a draft push to the bottom
	if pr.IsDraft {
		pr.Importance = 1
		pr.ImportanceLookup["Draft"] = 1
		return
	}

	// if I am NOT the author
	if !pr.IAmAuthor {
		importance += 100
		pr.ImportanceLookup["Not mine"] = 100
	}

	// if author is teammate add 50
	if pr.AuthorIsTeammate {
		importance += 100
		pr.ImportanceLookup["Teammate"] = 100
	}

	// if the author is not a bot subtract 50
	if pr.AuthorIsBot {
		importance -= 50
		pr.ImportanceLookup["Not bot"] = -50
	}

	// if the PR state is success
	if pr.JobStateSuccess {
		importance += 250
		pr.ImportanceLookup["Job State"] = 250
	}

	// if I have already viewed the PR
	if pr.ViewedAt != nil {
		importance -= 100
		pr.ImportanceLookup["Viewed"] = -100
	}

	// TODO : if there are no reviews this shows as cant merge.
	// this should look at status checks instead
	// if the PR cannot be merged sub 1000
	// if pr.Mergeable {
	// 	importance -= 250
	// 	pr.ImportanceLookup["Cant Merge"] = -100
	// }

	// More lines of code changed, the less important.
	// THis is clamped between 0 and 100
	codeImp := (1 / (float64(pr.CodeDelta) + 100)) * 1000
	clampedCodeImp := clampFloat(codeImp, 0, 100)
	importance += clampedCodeImp
	pr.ImportanceLookup["Code delta"] = clampedCodeImp

	// more requested reviewers the less important
	if !pr.IAmAuthor {
		revCount := float64(len(pr.RequestedReviewers))
		imp := ((1 / (revCount + 5)) * 1000) * 0.5
		clampedImp := clampFloat(imp, 0, 100)
		importance += clampedImp
		pr.ImportanceLookup["Reviewer count"] = clampedImp
	}

	// removing for now. this seems to give a bad signal
	// total pr age in min ((50/7500)*PR_AGE_MIN+25)
	//prAgeMin := time.Now().Sub(*pr.GithubPR.CreatedAt) / time.Minute
	//importance += float64(((50 / 7500) * prAgeMin) + 25)

	// if I am NOT the author but i have commented
	// and there are comments or commits after mine
	if !pr.IAmAuthor && pr.HasCommentsFromMe {
		// awaiting comment reply
		if !pr.LastCommentFromMe {
			minSinceLastComment := pr.TimeSinceLastComment / time.Minute
			// at 24 hrs day the importance is 100
			imp := math.Pow(float64(minSinceLastComment), 2) / 20000
			clampedImp := clampFloat(imp, 0, 300) // push this signal up high
			importance += clampedImp
			pr.ImportanceLookup["Awaiting reply"] = clampedImp
		}

		// new changes since my last comment
		if pr.LastCommentFromMeTime != nil && pr.LastCommitTime.After(*pr.LastCommentFromMeTime) {
			minSinceLastCommit := pr.TimeSinceLastCommit / time.Minute
			// at 48hrs hrs the importance is 100
			imp := math.Pow(float64(minSinceLastCommit), 2) / 80000
			clampedImp := clampFloat(imp, 0, 250) // 250 pushes this signal up
			importance += clampedImp
			pr.ImportanceLookup["New changes"] = clampedImp
		}
	}

	// If there are not any comments and I am not the author and there are commits
	// and the job state is success then this is ready but not been reviewed yet
	if !pr.IAmAuthor && len(pr.Comments) == 0 && pr.JobStateSuccess {
		minSinceLastCommit := pr.TimeSinceLastCommit / time.Minute
		// at 48hrs hrs the importance is 100
		imp := math.Pow(float64(minSinceLastCommit), 2) / 80000
		clampedImp := clampFloat(imp, 0, 50) // this should get looked at...
		importance += clampedImp
		pr.ImportanceLookup["First review"] = clampedImp
	}

	// min since last comment if I AM the author
	// i should rapidly respond to comments
	if pr.IAmAuthor && !pr.LastCommentFromMe {
		minSinceLastComment := pr.TimeSinceLastComment / time.Minute
		// at 24 hrs day the importance is 100
		imp := math.Pow(float64(minSinceLastComment), 2) / 20000
		clampedImp := clampFloat(imp, 0, 300) // push this signal up high
		importance += clampedImp
		pr.ImportanceLookup["Comment needed"] = clampedImp
	}

	pr.Importance = importance
}

func clampFloat(val float64, min float64, max float64) float64 {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}

func (ds *Datasource) RefreshPR(localPR *PullRequest) (returnedError error) {
	defer func() {
		if returnedError == nil {
			// always recalculate importance so time based signals are accurate
			localPR.calculateImportance(ds)
			ds.cachePR(localPR)
			ds.SavePRCacheToFile()
		}
	}()

	// if we previously pulled compare the timestamps to see if we need to
	// update other properties
	// if localPR.GithubPR != nil &&
	// 	shallowUpdate != nil &&
	// 	!shallowUpdate.GetUpdatedAt().Time.After((localPR.GithubPR.GetUpdatedAt().Time)) {
	// 	// no change in the PR we can skip updating
	// 	ds.writeStatus(fmt.Sprintf("%s/%s/%d no change", localPR.OrgName, localPR.RepoName, localPR.Number))
	// 	localPR.LastUpdatedAt = time.Now()
	// 	return nil
	// }

	currentGHPRData, err := ds.GetPull(localPR.OrgName, localPR.RepoName, localPR.Number)
	if err != nil {
		logger.Shared().Println("error getting pull request", err)
		ds.writeErrorStatus(err)
		return err
	}

	// dont make extra api calls if the PR is not open
	if currentGHPRData.GetState() != "open" {
		ds.writeStatus(fmt.Sprintf("%s/%s/%d closed", localPR.OrgName, localPR.RepoName, localPR.Number))
		localPR.GithubPR = currentGHPRData
		localPR.LastUpdatedAt = time.Now()
		return nil
	}

	// if the head sha does not match that means there are new commits. for now
	// pull the whole list of commits.
	if localPR.GithubPR == nil ||
		(currentGHPRData.GetCommits() > 0 && localPR.GithubPR.GetHead().GetSHA() != currentGHPRData.GetHead().GetSHA()) {
		ds.writeStatus(fmt.Sprintf("%s/%s/%d updating commits...", localPR.OrgName, localPR.RepoName, localPR.Number))
		commits, _, err := ds.GetAllCommitsForPull(localPR.OrgName, localPR.RepoName, localPR.Number, 0)
		if err != nil {
			ds.writeErrorStatus(err)
			return err
		}
		localPR.Commits = commits

		// // ugh...n^2 * 3 .... boooooo
		// // todo : refactor
		// for _, new := range commits {
		// 	found := false
		// 	for _, c := range localPR.Commits {
		// 		if c.GetNodeID() == new.GetNodeID() {
		// 			found = true
		// 			break
		// 		}
		// 	}
		// 	if !found {
		// 		localPR.Commits = append(localPR.Commits, new)
		// 	}
		// }
		// localPR.LastCommitsPage = lastCommitsPage
	}
	logger.Shared().Println(localPR.OrgName, localPR.RepoName, localPR.Number, localPR.GithubPR == nil, "|", currentGHPRData.GetCommits() > 0, localPR.GithubPR.GetCommits(), currentGHPRData.GetCommits())

	// if comment count doesn't match we need to update
	if localPR.GithubPR == nil || (currentGHPRData.GetComments() > 0 && currentGHPRData.GetUpdatedAt().Time.After(localPR.GithubPR.GetUpdatedAt().Time)) {
		ds.writeStatus(fmt.Sprintf("%s/%s/%d updating comments...", localPR.OrgName, localPR.RepoName, localPR.Number))
		comments, _, err := ds.GetAllCommentsForPull(localPR.OrgName, localPR.RepoName, localPR.Number, 0)
		if err != nil {
			ds.writeErrorStatus(err)
			return err
		}
		localPR.Comments = comments
		// for _, new := range comments {
		// 	found := false
		// 	for _, c := range localPR.Comments {
		// 		if c.GetNodeID() == new.GetNodeID() {
		// 			found = true
		// 			break
		// 		}
		// 	}
		// 	if !found {
		// 		localPR.Comments = append(localPR.Comments, new)
		// 	}
		// }
		// localPR.LastCommentsPage = lastCommentsPage
	}

	// if review comment count doesn't match we need to update
	if localPR.GithubPR == nil || (currentGHPRData.GetReviewComments() > 0 && currentGHPRData.GetUpdatedAt().Time.After(localPR.GithubPR.GetUpdatedAt().Time)) {
		ds.writeStatus(fmt.Sprintf("%s/%s/%d updating reviews...", localPR.OrgName, localPR.RepoName, localPR.Number))
		reviews, _, err := ds.GetAllReviewsForPull(localPR.OrgName, localPR.RepoName, localPR.Number, localPR.LastReviewsPage)
		if err != nil {
			ds.writeErrorStatus(err)
			return err
		}
		localPR.Reviews = reviews
		// for _, new := range reviews {
		// 	found := false
		// 	for _, c := range localPR.Reviews {
		// 		if c.GetNodeID() == new.GetNodeID() {
		// 			found = true
		// 			break
		// 		}
		// 	}
		// 	if !found {
		// 		localPR.Reviews = append(localPR.Reviews, new)
		// 	}
		// }
		// localPR.LastReviewsPage = lastReviewsPage
	}

	ds.writeStatus(fmt.Sprintf("%s/%s/%d fetching status...", localPR.OrgName, localPR.RepoName, localPR.Number))
	status, err := ds.GetCombinedStatus(localPR.OrgName, localPR.RepoName, currentGHPRData.GetHead().GetSHA())
	if err != nil {
		ds.writeErrorStatus(err)
		return err
	}
	localPR.Status = status

	// clear the viewed at because we refreshed the PR
	localPR.ViewedAt = nil

	// ensure we save the latest PR data from github
	localPR.GithubPR = currentGHPRData
	localPR.LastUpdatedAt = time.Now()

	return nil
}

func (ds *Datasource) UpdateExistingPr(org string, repo string, pr *PullRequest, ghpr *github.PullRequest) error {
	// check if the PR has been updated
	if !ghpr.GetUpdatedAt().Time.After(pr.GithubPR.GetUpdatedAt().Time) {
		// no change in the PR we can skip updating
		ds.writeStatus(fmt.Sprintf("%s/%s/%d no change", org, repo, *ghpr.Number))
		// recalculate importance so time based signals are accurate
		pr.calculateImportance(ds)
		return nil
	}

	// check if new commits
	if pr.GithubPR.GetCommits() != ghpr.GetCommits() && ghpr.GetCommits() > 0 {
		ds.writeStatus(fmt.Sprintf("%s/%s/%d updating commits...", org, repo, *ghpr.Number))
		commits, lastCommitsPage, err := ds.GetAllCommitsForPull(org, repo, *ghpr.Number, pr.LastCommitsPage)
		if err != nil {
			ds.writeErrorStatus(err)
			return err
		}
		// ugh...n^2 * 3 .... boooooo
		// todo : refactor
		for _, new := range commits {
			found := false
			for _, c := range pr.Commits {
				if c.GetNodeID() == new.GetNodeID() {
					found = true
					break
				}
			}
			if !found {
				pr.Commits = append(pr.Commits, new)
			}
		}
		pr.LastCommitsPage = lastCommitsPage
	}

	// check if new comments
	if pr.GithubPR.GetComments() != ghpr.GetComments() && ghpr.GetComments() > 0 {
		ds.writeStatus(fmt.Sprintf("%s/%s/%d updating comments...", org, repo, *ghpr.Number))
		comments, lastCommentsPage, err := ds.GetAllCommentsForPull(org, repo, *ghpr.Number, pr.LastCommentsPage)
		if err != nil {
			ds.writeErrorStatus(err)
			return err
		}
		for _, new := range comments {
			found := false
			for _, c := range pr.Comments {
				if c.GetNodeID() == new.GetNodeID() {
					found = true
					break
				}
			}
			if !found {
				pr.Comments = append(pr.Comments, new)
			}
		}
		pr.LastCommentsPage = lastCommentsPage
	}
	// check if new review comments
	if pr.GithubPR.GetReviewComments() != ghpr.GetReviewComments() && ghpr.GetReviewComments() > 0 {
		ds.writeStatus(fmt.Sprintf("%s/%s/%d updating reviews...", org, repo, *ghpr.Number))
		reviews, lastReviewsPage, err := ds.GetAllReviewsForPull(org, repo, *ghpr.Number, pr.LastReviewsPage)
		if err != nil {
			ds.writeErrorStatus(err)
			return err
		}
		for _, new := range reviews {
			found := false
			for _, c := range pr.Reviews {
				if c.GetNodeID() == new.GetNodeID() {
					found = true
					break
				}
			}
			if !found {
				pr.Reviews = append(pr.Reviews, new)
			}
		}
		pr.LastReviewsPage = lastReviewsPage
	}

	ds.writeStatus(fmt.Sprintf("%s/%s/%d fetching status...", org, repo, *ghpr.Number))
	status, err := ds.GetCombinedStatus(org, repo, ghpr.GetHead().GetSHA())
	if err != nil {
		ds.writeErrorStatus(err)
		return err
	}
	pr.Status = status

	// Store a reference to the github PR on our envelope for easy access later
	fullGHPR, err := ds.GetPull(org, repo, *ghpr.Number)
	if err != nil {
		ds.writeErrorStatus(err)
		return err
	}
	// ensure we have the latest extended PR info (additions/deletions)
	pr.GithubPR = fullGHPR

	pr.calculateImportance(ds)
	return nil
}

func (ds *Datasource) BuildPullRequest(org string, repo string, ghpr *github.PullRequest) (*PullRequest, error) {
	if ghpr == nil {
		return nil, fmt.Errorf("nil pull request passed to BuildPullRequest")
	}

	fullGHPR, err := ds.GetPull(org, repo, *ghpr.Number)
	if err != nil {
		ds.writeErrorStatus(err)
		return nil, err
	}
	if fullGHPR.GetState() == "closed" {
		return nil, fmt.Errorf("pull request is closed %s/%s/%d", org, repo, ghpr.GetNumber())
	}

	ds.writeStatus(fmt.Sprintf("%s/%s/%d fetching commits...", org, repo, *ghpr.Number))
	commits, lastCommitsPage, err := ds.GetAllCommitsForPull(org, repo, *ghpr.Number, 0)
	if err != nil {
		ds.writeErrorStatus(err)
		return nil, err
	}
	ds.writeStatus(fmt.Sprintf("%s/%s/%d fetching comments...", org, repo, *ghpr.Number))
	comments, lastCommentsPage, err := ds.GetAllCommentsForPull(org, repo, *ghpr.Number, 0)
	if err != nil {
		ds.writeErrorStatus(err)
		return nil, err
	}
	ds.writeStatus(fmt.Sprintf("%s/%s/%d fetching reviews...", org, repo, *ghpr.Number))
	reviews, lastReviewsPage, err := ds.GetAllReviewsForPull(org, repo, *ghpr.Number, 0)
	if err != nil {
		ds.writeErrorStatus(err)
		return nil, err
	}

	ds.writeStatus(fmt.Sprintf("%s/%s/%d fetching status...", org, repo, *ghpr.Number))
	status, err := ds.GetCombinedStatus(org, repo, ghpr.GetHead().GetSHA())
	if err != nil {
		ds.writeErrorStatus(err)
		return nil, err
	}

	newPR := &PullRequest{
		GithubPR: fullGHPR,
		OrgName:  org,
		RepoName: repo,
		Commits:  commits,
		Comments: comments,
		Reviews:  reviews,
		Status:   status,

		LastCommitsPage:  lastCommitsPage,
		LastCommentsPage: lastCommentsPage,
		LastReviewsPage:  lastReviewsPage,
	}
	newPR.calculateImportance(ds)

	return newPR, nil
}

func (ds *Datasource) GetAllPullsForRepoInOrg(orgName string, repoName string) ([]*github.PullRequest, error) {
	ctx := context.Background()
	opt := &github.PullRequestListOptions{
		ListOptions: github.ListOptions{PerPage: GHResultsPerPage},
		State:       "open",
	}
	// page through to get all pulls for this repo
	// NOTE: the returned PR data is not complete and needs to be directly fetched
	// to hydrate fields like additions and deletions
	var allPulls []*github.PullRequest
	for {
		logger.Shared().Printf("getting pulls for: [%s/%s] page:%d", orgName, repoName, opt.Page)
		prs, resp, err := sharedClient().PullRequests.List(ctx, orgName, repoName, opt)

		if _, ok := err.(*github.RateLimitError); ok {
			logger.Shared().Printf("pulls: hit rate limit [%s/%s]", orgName, repoName)
			untilReset := resp.Rate.Reset.Sub(time.Now())
			minUntilReset := untilReset.Round(time.Minute) / time.Minute
			ds.writeStatus(fmt.Sprintf("hit rate limit, waiting %02dm", minUntilReset))
			time.Sleep(untilReset)
			continue
		} else if arlerr, ok := err.(*github.AbuseRateLimitError); ok {
			logger.Shared().Printf("pulls: hit secondary rate limit [%s/%s]", orgName, repoName)
			time.Sleep(*arlerr.RetryAfter)
			continue
		} else if err != nil {
			logger.Shared().Printf("error getting pulls in [%s/%s] %s", orgName, repoName, err)
			return allPulls, err
		}

		ds.remainingRequestsChan <- resp.Rate
		allPulls = append(allPulls, prs...)
		if resp.NextPage == 0 || opt.Page == resp.NextPage {
			break
		}
		opt.Page = resp.NextPage
	}
	return allPulls, nil
}

func (ds *Datasource) GetPull(org string, repo string, prNumber int) (*github.PullRequest, error) {
	ctx := context.Background()
	fullPR, _, err := sharedClient().PullRequests.Get(ctx, org, repo, prNumber)
	return fullPR, err
}

func (ds *Datasource) GetAllCommitsForPull(org string, repo string, prNumber int, lastPage int) ([]*github.RepositoryCommit, int, error) {
	ctx := context.Background()
	opt := &github.ListOptions{
		PerPage: GHResultsPerPage,
		Page:    lastPage,
	}
	// get all pages of results
	var allCommits []*github.RepositoryCommit
	for {
		logger.Shared().Printf("commits: %s/%s/%d p:%d", org, repo, prNumber, opt.Page)
		commits, resp, err := sharedClient().PullRequests.ListCommits(ctx, org, repo, prNumber, opt)
		if _, ok := err.(*github.RateLimitError); ok {
			logger.Shared().Println("commits: hit rate limit")
			untilReset := resp.Rate.Reset.Sub(time.Now())
			minUntilReset := untilReset.Round(time.Minute) / time.Minute
			ds.writeStatus(fmt.Sprintf("hit rate limit, waiting %02dm", minUntilReset))
			time.Sleep(untilReset)
			continue
		} else if arlerr, ok := err.(*github.AbuseRateLimitError); ok {
			logger.Shared().Println("commits: hit secondary rate limit")
			time.Sleep(*arlerr.RetryAfter)
			continue
		} else if err != nil {
			logger.Shared().Printf("commits: error %s", err)
			return allCommits, lastPage, err
		}
		ds.remainingRequestsChan <- resp.Rate
		allCommits = append(allCommits, commits...)
		if resp.NextPage == 0 || opt.Page == resp.NextPage {
			break
		}
		opt.Page = resp.NextPage
		lastPage = resp.NextPage
	}
	return allCommits, lastPage, nil
}

func (ds *Datasource) GetAllCommentsForPull(org string, repo string, prNumber int, lastPage int) ([]*github.PullRequestComment, int, error) {
	ctx := context.Background()
	opt := &github.PullRequestListCommentsOptions{
		ListOptions: github.ListOptions{
			PerPage: GHResultsPerPage,
			Page:    lastPage,
		},
	}
	// get all pages of results
	var allComments []*github.PullRequestComment
	for {
		logger.Shared().Printf("comments: %s/%s/%d p:%d", org, repo, prNumber, opt.Page)
		comments, resp, err := sharedClient().PullRequests.ListComments(ctx, org, repo, prNumber, opt)
		if _, ok := err.(*github.RateLimitError); ok {
			logger.Shared().Println("comments: hit rate limit")
			untilReset := resp.Rate.Reset.Sub(time.Now())
			minUntilReset := untilReset.Round(time.Minute) / time.Minute
			ds.writeStatus(fmt.Sprintf("hit rate limit, waiting %02dm", minUntilReset))
			time.Sleep(untilReset)
			continue
		} else if arlerr, ok := err.(*github.AbuseRateLimitError); ok {
			logger.Shared().Println("comments: hit secondary rate limit")
			time.Sleep(*arlerr.RetryAfter)
			continue
		} else if err != nil {
			logger.Shared().Printf("comments: error %s", err)
			return allComments, lastPage, err
		}
		ds.remainingRequestsChan <- resp.Rate
		allComments = append(allComments, comments...)
		if resp.NextPage == 0 || opt.Page == resp.NextPage {
			break
		}
		opt.Page = resp.NextPage
		lastPage = resp.NextPage
	}
	return allComments, lastPage, nil
}

func (ds *Datasource) GetAllReviewsForPull(org string, repo string, prNumber int, lastPage int) ([]*github.PullRequestReview, int, error) {
	ctx := context.Background()
	opt := &github.RepositoryListByOrgOptions{
		ListOptions: github.ListOptions{
			PerPage: GHResultsPerPage,
			Page:    lastPage,
		},
		Type: "all",
	}
	// get all pages of results
	var allReviews []*github.PullRequestReview
	for {
		logger.Shared().Printf("reviews: %s/%s/%d p:%d", org, repo, prNumber, opt.Page)
		reviews, resp, err := sharedClient().PullRequests.ListReviews(ctx, org, repo, prNumber, nil)
		if _, ok := err.(*github.RateLimitError); ok {
			logger.Shared().Println("reviews: hit rate limit")
			untilReset := resp.Rate.Reset.Sub(time.Now())
			minUntilReset := untilReset.Round(time.Minute) / time.Minute
			ds.writeStatus(fmt.Sprintf("hit rate limit, waiting %02dm", minUntilReset))
			time.Sleep(untilReset)
			continue
		} else if arlerr, ok := err.(*github.AbuseRateLimitError); ok {
			logger.Shared().Println("reviews: hit secondary rate limit")
			time.Sleep(*arlerr.RetryAfter)
			continue
		} else if err != nil {
			logger.Shared().Printf("reviews: error %s", err)
			return allReviews, lastPage, err
		}
		allReviews = append(allReviews, reviews...)
		if resp.NextPage == 0 || opt.Page == resp.NextPage {
			break
		}
		opt.Page = resp.NextPage
		lastPage = resp.NextPage
	}
	return allReviews, lastPage, nil
}

func (ds *Datasource) GetCombinedStatus(org string, repo string, commitSha string) (*github.CombinedStatus, error) {
	ctx := context.Background()
	opt := &github.ListOptions{
		PerPage: GHResultsPerPage,
	}

	statuses, _, err := sharedClient().Repositories.GetCombinedStatus(ctx, org, repo, commitSha, opt)

	return statuses, err
}
