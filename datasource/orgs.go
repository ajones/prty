package datasource

import (
	"context"

	"github.com/google/go-github/v34/github"
)

var currentOrgs []*github.Organization

func GetAllOrgs() ([]*github.Organization, error) {
	ctx := context.Background()
	opt := &github.ListOptions{PerPage: 10}
	// get all pages of results
	var allOrgs []*github.Organization
	for {
		reviews, resp, err := sharedClient().Organizations.List(ctx, "", opt)
		if err != nil {
			return allOrgs, err
		}
		allOrgs = append(allOrgs, reviews...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	return allOrgs, nil
}
