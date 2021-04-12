package datasource

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/google/go-github/github"
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

	Importance float64
}

func (pr *PullRequest) calculateStatusFields(orgName string) {
	pr.Author = *pr.PR.User.Login
	pr.OrgName = orgName
	pr.RepoName = *pr.PR.Head.Repo.Name
	pr.NumCommits = len(pr.Commits)

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

func (d *Datasource) calculateImportance(pr *PullRequest) {
	pr.Importance = 0

	// 0. if I am not the author and it is approved we dont need to look at it
	if pr.Author != d.myUsername && pr.IsApproved {
		return
	}
	// 0. if this pr is abandoned then drop it to the bottom
	if pr.IsAbandoned {
		return
	}

	// 1. if I am NOT the author add 50
	if pr.Author != d.myUsername {
		pr.Importance += 50
	}

	// 2. if author is teammate add 50
	for _, n := range d.myTeamUsernames {
		if n == pr.Author {
			pr.Importance += 50
			break
		}
	}

	// 3. if I am NOT the author (100-POW(NUM_REQUESTED_REVIEWERS,2))
	if pr.Author != d.myUsername {
		revCount := float64(len(pr.RequestedReviewers))
		pr.Importance += (100 - math.Pow(revCount, 2))
	}

	// 4. total pr age in min ((50/7500)*PR_AGE_MIN+25)
	prAgeMin := time.Now().Sub(*pr.PR.CreatedAt) / time.Minute
	pr.Importance += float64(((50 / 7500) * prAgeMin) + 25)

	// 5. min since last commit. if I am NOT the author ((100/7500)*MIN_SINCE_LAST_COMMIT+50)
	if pr.Author != d.myUsername {
		minSinceLastCommit := time.Now().Sub(pr.LastCommitTime) / time.Minute
		pr.Importance += float64((100/7500)*minSinceLastCommit + 50)
	}

	// 6. min since last comment if I AM the author ((600/7500)*MIN_SINCE_LAST_COMMENT+50)
	if pr.Author == d.myUsername {
		minSinceLastComment := time.Now().Sub(pr.LastCommentTime) / time.Minute
		pr.Importance += float64((100/7500)*minSinceLastComment + 50)
	}

	// 7. if it is mine and approved we should go look at it
	if pr.Author == d.myUsername && pr.IsApproved {
		pr.Importance += float64(prAgeMin)
	}
}

func (d *Datasource) OnUpdatedPulls(org string, repo string, prs []*github.PullRequest) error {
	processed := []*PullRequest{}
	for i := range prs {
		ghpr := prs[i]
		ctx := context.Background()

		// TODO : ADD PAGING TO ALL THESE!!!!
		d.writeStatus(fmt.Sprintf("%s/%s/#%d fetching commits...", org, repo, *ghpr.Number))
		commits, _, _ := sharedClient().PullRequests.ListCommits(ctx, org, repo, *ghpr.Number, nil)
		d.writeStatus(fmt.Sprintf("%s/%s/#%d fetching comments...", org, repo, *ghpr.Number))
		comments, _, _ := sharedClient().PullRequests.ListComments(ctx, org, repo, *ghpr.Number, nil)
		d.writeStatus(fmt.Sprintf("%s/%s/#%d fetching reviews...", org, repo, *ghpr.Number))
		reviews, _, _ := sharedClient().PullRequests.ListReviews(ctx, org, repo, *ghpr.Number, nil)

		newPR := &PullRequest{
			PR:       ghpr,
			Commits:  commits,
			Comments: comments,
			Reviews:  reviews,
		}
		newPR.calculateStatusFields(org)
		d.calculateImportance(newPR)
		processed = append(processed, newPR)
	}
	currentPulls = append(currentPulls, processed...)
	sort.Sort(byImportance(currentPulls))
	return nil
}

func GetAllPulls() ([]*PullRequest, error) {
	return currentPulls, nil
	/*
		allPulls := []*PullRequest{}
		for _, list := range currentPulls {
			allPulls = append(allPulls, list...)
		}
		return allPulls, nil
	*/
}

type byImportance []*PullRequest

func (s byImportance) Len() int {
	return len(s)
}
func (s byImportance) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s byImportance) Less(i, j int) bool {
	return s[i].Importance > s[j].Importance
}

/*
func GetAllPulls(org string, repo string) ([]*PullRequest, error) {
	if len(currentPulls) > 0 {
		return currentPulls, nil
	}

	ctx := context.Background()
	prs, _, _ := sharedClient().PullRequests.List(ctx, org, repo, nil)

	for i := range prs {
		ghpr := prs[i]
		commits, _, _ := sharedClient().PullRequests.ListCommits(ctx, org, repo, *ghpr.Number, nil)
		comments, _, _ := sharedClient().PullRequests.ListComments(ctx, org, repo, *ghpr.Number, nil)
		reviews, _, _ := sharedClient().PullRequests.ListReviews(ctx, org, repo, *ghpr.Number, nil)

		newPR := &PullRequest{
			PR:       ghpr,
			Commits:  commits,
			Comments: comments,
			Reviews:  reviews,
		}
		newPR.calculateStatusFields()
		currentPulls = append(currentPulls, newPR)
	}

	return currentPulls, nil
}

func GetPullsForRepo(org string, repo string) ([]*PullRequest, error) {
	ctx := context.Background()
	prs, _, _ := sharedClient().PullRequests.List(ctx, org, repo, nil)

	pulls := []*PullRequest{}
	for i := range prs {
		ghpr := prs[i]
		commits, _, _ := sharedClient().PullRequests.ListCommits(ctx, org, repo, *ghpr.Number, nil)
		comments, _, _ := sharedClient().PullRequests.ListComments(ctx, org, repo, *ghpr.Number, nil)
		reviews, _, _ := sharedClient().PullRequests.ListReviews(ctx, org, repo, *ghpr.Number, nil)

		newPR := &PullRequest{
			PR:       ghpr,
			Commits:  commits,
			Comments: comments,
			Reviews:  reviews,
		}
		newPR.calculateStatusFields()
		pulls = append(pulls, newPR)
	}

	return pulls, nil
}
*/
