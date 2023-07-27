package datasource

import (
	"context"

	"github.com/google/go-github/v53/github"
	"github.com/inburst/prty/logger"
)

func GetAllOrgs() ([]*github.Organization, error) {
	ctx := context.Background()
	opt := &github.ListOptions{PerPage: 10}
	// get all pages of results
	var allOrgs []*github.Organization
	for {
		orgs, resp, err := sharedClient().Organizations.List(ctx, "", opt)
		if err != nil {
			logger.Shared().Printf("org err: %s\n", err)
			return allOrgs, err
		}

		logger.Shared().Printf("found orgs: count:%d\n", len(orgs))
		allOrgs = append(allOrgs, orgs...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	return allOrgs, nil
}

func GetOrg(orgName string) (*github.Organization, error) {
	ctx := context.Background()

	org, _, err := sharedClient().Organizations.Get(ctx, orgName)
	if err != nil {
		logger.Shared().Printf("public org err: %s\n", err)
		return nil, err
	}
	return org, nil
}
