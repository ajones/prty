package datasource

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/google/go-github/v53/github"
	"github.com/inburst/prty/logger"
)

var currentPulls []*PullRequest = []*PullRequest{}

type PullRequest struct {
	PR               *github.PullRequest
	Commits          []*github.RepositoryCommit
	Comments         []*github.PullRequestComment
	Reviews          []*github.PullRequestReview
	LastCommitsPage  int
	LastCommentsPage int
	LastReviewsPage  int

	Author             string
	RepoName           string
	OrgName            string
	Labels             []string
	RequestedReviewers []string
	NumCommits         int

	IAmAuthor                  bool
	AuthorIsTeammate           bool
	AuthorIsBot                bool
	HasChangesAfterLastComment bool
	HasCommentsFromMe          bool
	LastCommentFromMe          bool
	LastCommentTime            time.Time
	LastCommitTime             time.Time
	FirstCommitTime            time.Time
	IsApproved                 bool
	IsAbandoned                bool
	IsDraft                    bool
	Additions                  int
	Deletions                  int
	CodeDelta                  int

	Importance       float64
	ImportanceLookup map[string]float64

	ViewedAt *time.Time
}

func (pr *PullRequest) calculateStatusFields(orgName string, repoName string, ds *Datasource) {
	pr.Author = pr.PR.User.GetLogin()
	pr.OrgName = orgName
	pr.RepoName = repoName
	pr.NumCommits = len(pr.Commits)
	pr.IsDraft = pr.PR.GetDraft()
	pr.IAmAuthor = (pr.Author == ds.config.GithubUsername)
	pr.Additions = pr.PR.GetAdditions()
	pr.Deletions = pr.PR.GetDeletions()
	pr.CodeDelta = pr.Additions + pr.Deletions

	for _, n := range ds.config.TeamUsernames {
		if n == pr.Author {
			pr.AuthorIsTeammate = true
			break
		}
	}

	for _, n := range ds.config.BotUsernames {
		if n == pr.Author {
			pr.AuthorIsBot = true
			break
		}
	}

	for i, c := range pr.Comments {
		if i == 0 || pr.LastCommentTime.Before(*c.CreatedAt.GetTime()) {
			pr.LastCommentTime = *c.CreatedAt.GetTime()

			// mark if the most recent comment is from me
			if c.User.GetLogin() == ds.config.GithubUsername {
				pr.LastCommentFromMe = true
			}
		}

		// mark if I have commented on this PR
		if c.User.GetLogin() == ds.config.GithubUsername {
			pr.HasCommentsFromMe = true
			break
		}
	}

	for _, c := range pr.Commits {
		if pr.FirstCommitTime.IsZero() {
			pr.FirstCommitTime = *c.Commit.Committer.Date.GetTime()
		}
		if pr.LastCommitTime.IsZero() {
			pr.LastCommitTime = *c.Commit.Committer.Date.GetTime()
		}

		if pr.FirstCommitTime.After(*c.Commit.Committer.Date.GetTime()) {
			pr.FirstCommitTime = *c.Commit.Committer.Date.GetTime()
		}
		if pr.LastCommitTime.Before(*c.Commit.Committer.Date.GetTime()) {
			pr.LastCommitTime = *c.Commit.Committer.Date.GetTime()
		}
	}
	pr.IsAbandoned = time.Now().After(pr.LastCommitTime.Add(time.Duration(21) * time.Hour * time.Duration(24)))

	// clear viewed at if there are new changes
	if pr.ViewedAt != nil {
		if pr.LastCommitTime.After(*pr.ViewedAt) || pr.LastCommentTime.After(*pr.ViewedAt) {
			pr.ViewedAt = nil
		}
	}

	/*
		// TODO : PULL THIS OUT TO NOT BE READ EVERY TIME!!!
		abandonAgeDays := os.Getenv("ABANDONED_AGE_DAYS")
		if days, err := strconv.Atoi(abandonAgeDays); err == nil {
			pr.IsAbandoned = time.Now().After(pr.LastCommitTime.Add(time.Duration(21) * time.Hour * time.Duration(24)))
		}
	*/

	// flag if we have unreviewed changes
	if len(pr.Comments) == 0 || pr.LastCommentTime.Before(pr.LastCommitTime) {
		pr.HasChangesAfterLastComment = true
	}

	for i := range pr.PR.Labels {
		l := pr.PR.Labels[i]
		pr.Labels = append(pr.Labels, *l.Name)
	}

	for i := range pr.PR.RequestedReviewers {
		r := pr.PR.RequestedReviewers[i]
		pr.RequestedReviewers = append(pr.RequestedReviewers, *r.Login)
	}

	for _, r := range pr.Reviews {
		if *r.State == "APPROVED" {
			pr.IsApproved = true
		}
	}
}

// All importance values are normilized to 0-100
func (pr *PullRequest) calculateImportance(ds *Datasource) {
	pr.ImportanceLookup = make(map[string]float64)
	importance := 0.0

	// TODOs:
	// add importance if i am CODEOWNER
	// Sort by most recent activity first.

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
		pr.ImportanceLookup["Reviewers"] = clampedImp
	}

	// removing for now. this seems to give a bad signal
	// total pr age in min ((50/7500)*PR_AGE_MIN+25)
	//prAgeMin := time.Now().Sub(*pr.PR.CreatedAt) / time.Minute
	//importance += float64(((50 / 7500) * prAgeMin) + 25)

	// min since last commit. if I am NOT the author but i have commented
	// this is a high importance signal
	if !pr.IAmAuthor && pr.HasCommentsFromMe {
		minSinceLastCommit := time.Since(pr.LastCommitTime) / time.Minute
		// at 48hrs hrs the importance is 100
		imp := math.Pow(float64(minSinceLastCommit), 2) / 80000
		clampedImp := clampFloat(imp, 0, 250) // 250 pushes this signal up
		importance += clampedImp
		pr.ImportanceLookup["Change replies"] = clampedImp
	}

	// min since last commit. if I am NOT the author and I haven't commented
	if !pr.IAmAuthor && !pr.HasCommentsFromMe {
		minSinceLastCommit := time.Since(pr.LastCommitTime) / time.Minute
		// at 48hrs hrs the importance is 100
		imp := math.Pow(float64(minSinceLastCommit), 2) / 80000
		clampedImp := clampFloat(imp, 0, 50) // this should get looked at...
		importance += clampedImp
		pr.ImportanceLookup["Recent changes"] = clampedImp
	}

	// min since last comment if I AM the author
	// i should rapidly respond to comments
	if pr.IAmAuthor && !pr.LastCommentFromMe {
		minSinceLastComment := time.Since(pr.LastCommentTime) / time.Minute
		// at 24 hrs day the importance is 100
		imp := math.Pow(float64(minSinceLastComment), 2) / 20000
		clampedImp := clampFloat(imp, 0, 300) // push this signal up high
		importance += clampedImp
		pr.ImportanceLookup["Recent comment"] = clampedImp
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

func (ds *Datasource) UpdateExistingPr(org string, repo string, pr *PullRequest, ghpr *github.PullRequest) error {
	pr.PR = ghpr

	ds.writeStatus(fmt.Sprintf("%s/%s/#%d fetching commits...", org, repo, *ghpr.Number))
	commits, lastCommitsPage, err := ds.GetAllCommitsForPull(org, repo, *ghpr.Number, pr.LastCommitsPage)
	if err != nil {
		ds.writeErrorStatus(err)
		return err
	}
	ds.writeStatus(fmt.Sprintf("%s/%s/#%d fetching comments...", org, repo, *ghpr.Number))
	comments, lastCommentsPage, err := ds.GetAllCommentsForPull(org, repo, *ghpr.Number, pr.LastCommentsPage)
	if err != nil {
		ds.writeErrorStatus(err)
		return err
	}
	ds.writeStatus(fmt.Sprintf("%s/%s/#%d fetching reviews...", org, repo, *ghpr.Number))
	reviews, lastReviewsPage, err := ds.GetAllReviewsForPull(org, repo, *ghpr.Number, pr.LastReviewsPage)
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

	pr.LastCommitsPage = lastCommitsPage
	pr.LastCommentsPage = lastCommentsPage
	pr.LastReviewsPage = lastReviewsPage

	pr.calculateStatusFields(org, repo, ds)
	pr.calculateImportance(ds)

	return nil
}

func (ds *Datasource) BuildPullRequest(org string, repo string, ghpr *github.PullRequest) (*PullRequest, error) {
	ds.writeStatus(fmt.Sprintf("%s/%s/#%d fetching commits...", org, repo, *ghpr.Number))
	commits, lastCommitsPage, err := ds.GetAllCommitsForPull(org, repo, *ghpr.Number, 0)
	if err != nil {
		ds.writeErrorStatus(err)
		return nil, err
	}
	ds.writeStatus(fmt.Sprintf("%s/%s/#%d fetching comments...", org, repo, *ghpr.Number))
	comments, lastCommentsPage, err := ds.GetAllCommentsForPull(org, repo, *ghpr.Number, 0)
	if err != nil {
		ds.writeErrorStatus(err)
		return nil, err
	}
	ds.writeStatus(fmt.Sprintf("%s/%s/#%d fetching reviews...", org, repo, *ghpr.Number))
	reviews, lastReviewsPage, err := ds.GetAllReviewsForPull(org, repo, *ghpr.Number, 0)
	if err != nil {
		ds.writeErrorStatus(err)
		return nil, err
	}

	newPR := &PullRequest{
		PR:       ghpr,
		Commits:  commits,
		Comments: comments,
		Reviews:  reviews,

		LastCommitsPage:  lastCommitsPage,
		LastCommentsPage: lastCommentsPage,
		LastReviewsPage:  lastReviewsPage,
	}
	newPR.calculateStatusFields(org, repo, ds)
	newPR.calculateImportance(ds)

	return newPR, nil
}

func (ds *Datasource) GetAllPullsForRepoInOrg(orgName string, repoName string) ([]*github.PullRequest, error) {
	ctx := context.Background()
	opt := &github.PullRequestListOptions{
		ListOptions: github.ListOptions{PerPage: 10},
		State:       "open",
	}
	// get all pages of results
	var allPulls []*github.PullRequest
	for {
		logger.Shared().Printf("getting pulls for: [%s/%s] page:%d", orgName, repoName, opt.Page)
		prs, resp, err := sharedClient().PullRequests.List(ctx, orgName, repoName, opt)
		if _, ok := err.(*github.RateLimitError); ok {
			logger.Shared().Printf("pulls for: [%s/%s] hit rate limit", orgName, repoName)
			// TODO : add jitter
			time.Sleep(time.Duration(5) * time.Second)
			continue
		}
		if err != nil {
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

func (ds *Datasource) GetAllCommitsForPull(org string, repo string, prNumber int, lastPage int) ([]*github.RepositoryCommit, int, error) {
	ctx := context.Background()
	opt := &github.ListOptions{
		PerPage: 10,
		Page:    lastPage,
	}
	// get all pages of results
	var allCommits []*github.RepositoryCommit
	for {
		logger.Shared().Printf("commits: %s/%s/%d p:%d", org, repo, prNumber, opt.Page)
		commits, resp, err := sharedClient().PullRequests.ListCommits(ctx, org, repo, prNumber, opt)
		if _, ok := err.(*github.RateLimitError); ok {
			logger.Shared().Println("commits: hit rate limit")
			time.Sleep(time.Duration(5) * time.Second)
			continue
		}
		if err != nil {
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
			PerPage: 10,
			Page:    lastPage,
		},
	}
	// get all pages of results
	var allComments []*github.PullRequestComment
	for {
		logger.Shared().Println(fmt.Sprintf("comments: %s/%s/%d p:%d", org, repo, prNumber, opt.Page))
		comments, resp, err := sharedClient().PullRequests.ListComments(ctx, org, repo, prNumber, opt)
		if _, ok := err.(*github.RateLimitError); ok {
			logger.Shared().Println("comments: hit rate limit")
			time.Sleep(time.Duration(5) * time.Second)
			continue
		}
		if err != nil {
			logger.Shared().Println(fmt.Sprintf("comments: error %s", err))
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
			PerPage: 10,
			Page:    lastPage,
		},
		Type: "all",
	}
	// get all pages of results
	var allReviews []*github.PullRequestReview
	for {
		logger.Shared().Println(fmt.Sprintf("reviews: %s/%s/%d p:%d", org, repo, prNumber, opt.Page))
		reviews, resp, err := sharedClient().PullRequests.ListReviews(ctx, org, repo, prNumber, nil)
		if _, ok := err.(*github.RateLimitError); ok {
			logger.Shared().Println("reviews: hit rate limit")
			time.Sleep(time.Duration(5) * time.Second)
			continue
		}
		if err != nil {
			logger.Shared().Println(fmt.Sprintf("reviews: error %s", err))
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
