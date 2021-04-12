package datasource

import (
	"context"
	"fmt"

	"github.com/google/go-github/github"
)

var currentRepos []*github.Repository

func GetAllReposForOrg(orgName string) ([]*github.Repository, error) {
	if len(currentRepos) > 0 {
		return currentRepos, nil
	}

	ctx := context.Background()

	opt := &github.RepositoryListByOrgOptions{
		ListOptions: github.ListOptions{PerPage: 10},
		Type:        "all",
	}
	// get all pages of results
	var allRepos []*github.Repository
	for {
		repos, resp, err := sharedClient().Repositories.ListByOrg(ctx, orgName, opt)
		if err != nil {
			fmt.Printf("%s", err)
			return currentRepos, err
		}
		allRepos = append(allRepos, repos...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	currentRepos = allRepos

	return currentRepos, nil
}
