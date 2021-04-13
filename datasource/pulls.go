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
	PR       *github.PullRequest
	Commits  []*github.RepositoryCommit
	Comments []*github.PullRequestComment
	Reviews  []*github.PullRequestReview

	Author             string
	RepoName           string
	OrgName            string
	Labels             []string
	RequestedReviewers []string
	NumCommits         int

	HasChangesAfterLastComment bool
	LastCommentTime            time.Time
	LastCommitTime             time.Time
	IsApproved                 bool
	IsAbandoned                bool
	IsDraft                    bool

	Importance float64

	ViewedAt *time.Time
}

func (pr *PullRequest) calculateStatusFields(orgName string) {
	pr.Author = *pr.PR.User.Login
	pr.OrgName = orgName
	pr.RepoName = *pr.PR.Head.Repo.Name
	pr.NumCommits = len(pr.Commits)
	pr.IsDraft = *pr.PR.Draft

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

func (d *Datasource) calculateImportance(pr *PullRequest) float64 {
	importance := 0.0

	// if I am not the author and it is approved we dont need to look at it
	if pr.Author != d.myUsername && pr.IsApproved {
		return importance
	}
	// if this pr is abandoned then drop it to the bottom
	if pr.IsAbandoned {
		return importance
	}
	// if this pr is a draft push to the bottom
	if pr.IsDraft {
		return 1
	}

	// 1. if I am NOT the author add 50
	if pr.Author != d.myUsername {
		importance += 50
	}

	// 2. if author is teammate add 50
	for _, n := range d.myTeamUsernames {
		if n == pr.Author {
			importance += 50
			break
		}
	}

	// 3. if I am NOT the author (100-POW(NUM_REQUESTED_REVIEWERS,2))
	if pr.Author != d.myUsername {
		revCount := float64(len(pr.RequestedReviewers))
		importance += (100 - math.Pow(revCount, 2))
	}

	// 4. total pr age in min ((50/7500)*PR_AGE_MIN+25)
	prAgeMin := time.Now().Sub(*pr.PR.CreatedAt) / time.Minute
	importance += float64(((50 / 7500) * prAgeMin) + 25)

	// 5. min since last commit. if I am NOT the author ((100/7500)*MIN_SINCE_LAST_COMMIT+50)
	if pr.Author != d.myUsername {
		minSinceLastCommit := time.Now().Sub(pr.LastCommitTime) / time.Minute
		importance += float64((100/7500)*minSinceLastCommit + 50)
	}

	// 6. min since last comment if I AM the author ((600/7500)*MIN_SINCE_LAST_COMMENT+50)
	if pr.Author == d.myUsername {
		minSinceLastComment := time.Now().Sub(pr.LastCommentTime) / time.Minute
		importance += float64((100/7500)*minSinceLastComment + 50)
	}

	// 7. if it is mine and approved we should go look at it
	if pr.Author == d.myUsername && pr.IsApproved {
		importance += float64(prAgeMin)
	}

	return importance
}

func (d *Datasource) BuildPullRequest(org string, repo string, ghpr *github.PullRequest) (*PullRequest, error) {
	d.writeStatus(fmt.Sprintf("%s/%s/#%d fetching commits...", org, repo, *ghpr.Number))
	commits, err := GetAllCommitsForPull(org, repo, *ghpr.Number)
	if err != nil {
		d.writeErrorStatus(err)
		return nil, err
	}
	d.writeStatus(fmt.Sprintf("%s/%s/#%d fetching comments...", org, repo, *ghpr.Number))
	comments, err := GetAllCommentsForPull(org, repo, *ghpr.Number)
	if err != nil {
		d.writeErrorStatus(err)
		return nil, err
	}
	d.writeStatus(fmt.Sprintf("%s/%s/#%d fetching reviews...", org, repo, *ghpr.Number))
	reviews, err := GetAllReviewsForPull(org, repo, *ghpr.Number)
	if err != nil {
		d.writeErrorStatus(err)
		return nil, err
	}

	newPR := &PullRequest{
		PR:       ghpr,
		Commits:  commits,
		Comments: comments,
		Reviews:  reviews,
	}
	newPR.calculateStatusFields(org)
	newPR.Importance = d.calculateImportance(newPR)

	return newPR, nil
}

func GetAllPullsForRepoInOrg(orgName string, repoName string) ([]*github.PullRequest, error) {
	ctx := context.Background()
	opt := &github.PullRequestListOptions{
		ListOptions: github.ListOptions{PerPage: 10},
		State:       "open",
	}
	// get all pages of results
	var allPulls []*github.PullRequest
	for {
		prs, resp, err := sharedClient().PullRequests.List(ctx, orgName, repoName, opt)
		if err != nil {
			fmt.Printf("%s", err)
			return allPulls, err
		}
		allPulls = append(allPulls, prs...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	return allPulls, nil
}

func GetAllCommitsForPull(org string, repo string, prNumber int) ([]*github.RepositoryCommit, error) {
	ctx := context.Background()
	opt := &github.ListOptions{PerPage: 10}
	// get all pages of results
	var allCommits []*github.RepositoryCommit
	for {
		commits, resp, err := sharedClient().PullRequests.ListCommits(ctx, org, repo, prNumber, opt)
		if err != nil {
			fmt.Printf("%s", err)
			return allCommits, err
		}
		allCommits = append(allCommits, commits...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	return allCommits, nil
}

func GetAllCommentsForPull(org string, repo string, prNumber int) ([]*github.PullRequestComment, error) {
	ctx := context.Background()
	opt := &github.PullRequestListCommentsOptions{
		ListOptions: github.ListOptions{PerPage: 10},
	}
	// get all pages of results
	var allComments []*github.PullRequestComment
	for {
		comments, resp, err := sharedClient().PullRequests.ListComments(ctx, org, repo, prNumber, opt)
		if err != nil {
			fmt.Printf("%s", err)
			return allComments, err
		}
		allComments = append(allComments, comments...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	return allComments, nil
}

func GetAllReviewsForPull(org string, repo string, prNumber int) ([]*github.PullRequestReview, error) {
	ctx := context.Background()
	opt := &github.RepositoryListByOrgOptions{
		ListOptions: github.ListOptions{PerPage: 10},
		Type:        "all",
	}
	// get all pages of results
	var allReviews []*github.PullRequestReview
	for {
		reviews, resp, err := sharedClient().PullRequests.ListReviews(ctx, org, repo, prNumber, nil)
		if err != nil {
			fmt.Printf("%s", err)
			return allReviews, err
		}
		allReviews = append(allReviews, reviews...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	return allReviews, nil
}
