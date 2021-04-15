package datasource

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/google/go-github/v34/github"
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
	HasChangesAfterLastComment bool
	LastCommentTime            time.Time
	LastCommitTime             time.Time
	IsApproved                 bool
	IsAbandoned                bool
	IsDraft                    bool

	Importance float64

	ViewedAt *time.Time
}

func (pr *PullRequest) calculateStatusFields(orgName string, repoName string, ds *Datasource) {
	pr.Author = *pr.PR.User.Login
	pr.OrgName = orgName
	pr.RepoName = repoName
	pr.NumCommits = len(pr.Commits)
	pr.IsDraft = *pr.PR.Draft
	pr.IAmAuthor = (pr.Author == ds.config.GithubUsername)

	for _, n := range ds.config.TeamUsernames {
		if n == pr.Author {
			pr.AuthorIsTeammate = true
			break
		}
	}

	for i, c := range pr.Comments {
		if i == 0 || pr.LastCommentTime.Before(*c.CreatedAt) {
			pr.LastCommentTime = *c.CreatedAt
		}
	}

	for i, c := range pr.Commits {
		if i == 0 || pr.LastCommitTime.Before(*c.Commit.Committer.Date) {
			pr.LastCommitTime = *c.Commit.Committer.Date
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

func (pr *PullRequest) calculateImportance(ds *Datasource) {
	importance := 0.0

	// TODO: add importance if i am CODEOWNER

	// if I am not the author and it is approved we dont need to look at it
	if pr.Author != ds.config.GithubUsername && pr.IsApproved {
		return
	}
	// if this pr is abandoned then drop it to the bottom
	if pr.IsAbandoned {
		return
	}
	// if this pr is a draft push to the bottom
	if pr.IsDraft {
		pr.Importance = 1
		return
	}

	// 1. if I am NOT the author add 50
	if !pr.IAmAuthor {
		importance += 50
	}

	// 2. if author is teammate add 50
	if pr.AuthorIsTeammate {
		importance += 50
	}

	// 3. if I am NOT the author (100-POW(NUM_REQUESTED_REVIEWERS,2))
	if pr.Author != ds.config.GithubUsername {
		revCount := float64(len(pr.RequestedReviewers))
		importance += (100 - math.Pow(revCount, 2))
	}

	// 4. total pr age in min ((50/7500)*PR_AGE_MIN+25)
	prAgeMin := time.Now().Sub(*pr.PR.CreatedAt) / time.Minute
	importance += float64(((50 / 7500) * prAgeMin) + 25)

	// 5. min since last commit. if I am NOT the author ((100/7500)*MIN_SINCE_LAST_COMMIT+50)
	if pr.Author != ds.config.GithubUsername {
		minSinceLastCommit := time.Now().Sub(pr.LastCommitTime) / time.Minute
		importance += float64((100/7500)*minSinceLastCommit + 50)
	}

	// 6. min since last comment if I AM the author ((600/7500)*MIN_SINCE_LAST_COMMENT+50)
	if pr.Author == ds.config.GithubUsername {
		minSinceLastComment := time.Now().Sub(pr.LastCommentTime) / time.Minute
		importance += float64((100/7500)*minSinceLastComment + 50)
	}

	// 7. if it is mine and approved we should go look at it
	if pr.Author == ds.config.GithubUsername && pr.IsApproved {
		importance += float64(prAgeMin)
	}

	pr.Importance = importance
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
		println(fmt.Sprintf("pulls: %s/%s p:%d", orgName, repoName, opt.Page))
		prs, resp, err := sharedClient().PullRequests.List(ctx, orgName, repoName, opt)
		if _, ok := err.(*github.RateLimitError); ok {
			println("pulls: hit rate limit")
			time.Sleep(time.Duration(5) * time.Second)
			continue
		}
		if err != nil {
			println(fmt.Sprintf("pulls: error %s", err))
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
		println(fmt.Sprintf("commits: %s/%s/%d p:%d", org, repo, prNumber, opt.Page))
		commits, resp, err := sharedClient().PullRequests.ListCommits(ctx, org, repo, prNumber, opt)
		if _, ok := err.(*github.RateLimitError); ok {
			println("commits: hit rate limit")
			time.Sleep(time.Duration(5) * time.Second)
			continue
		}
		if err != nil {
			println(fmt.Sprintf("commits: error %s", err))
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
		println(fmt.Sprintf("comments: %s/%s/%d p:%d", org, repo, prNumber, opt.Page))
		comments, resp, err := sharedClient().PullRequests.ListComments(ctx, org, repo, prNumber, opt)
		if _, ok := err.(*github.RateLimitError); ok {
			println("comments: hit rate limit")
			time.Sleep(time.Duration(5) * time.Second)
			continue
		}
		if err != nil {
			println(fmt.Sprintf("comments: error %s", err))
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
		println(fmt.Sprintf("reviews: %s/%s/%d p:%d", org, repo, prNumber, opt.Page))
		reviews, resp, err := sharedClient().PullRequests.ListReviews(ctx, org, repo, prNumber, nil)
		if _, ok := err.(*github.RateLimitError); ok {
			println("reviews: hit rate limit")
			time.Sleep(time.Duration(5) * time.Second)
			continue
		}
		if err != nil {
			println(fmt.Sprintf("reviews: error %s", err))
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
