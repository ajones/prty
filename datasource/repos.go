package datasource

import (
	"context"

	"github.com/google/go-github/v34/github"
)

func GetAllReposForOrg(orgName string) ([]*github.Repository, error) {
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
			return allRepos, err
		}
		allRepos = append(allRepos, repos...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	return allRepos, nil
}
