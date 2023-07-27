package datasource

import (
	"context"

	"github.com/google/go-github/v53/github"
	"github.com/inburst/prty/logger"
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
			logger.Shared().Printf("repos err for org [%s]: %s\n", orgName, err)
			return allRepos, err
		}
		logger.Shared().Printf("found repos in org [%s]: count:%d\n", orgName, len(repos))
		allRepos = append(allRepos, repos...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	return allRepos, nil
}

func GetRepoInOrg(orgName string, repoName string) (*github.Repository, error) {
	ctx := context.Background()
	repo, _, err := sharedClient().Repositories.Get(ctx, orgName, repoName)
	if err != nil {
		logger.Shared().Printf("repo err for [%s/%s]: %s\n", orgName, repoName, err)
		return nil, err
	}
	logger.Shared().Printf("found repos [%s/%s]\n", orgName, repoName)
	return repo, nil
}
