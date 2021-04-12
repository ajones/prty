package datasource

import (
	"context"

	"github.com/google/go-github/github"
)

var currentOrgs []*github.Organization

func GetAllOrgs() ([]*github.Organization, error) {
	if len(currentOrgs) > 0 {
		return currentOrgs, nil
	}

	ctx := context.Background()
	orgs, _, err := sharedClient().Organizations.List(ctx, "", nil)
	currentOrgs = orgs
	return orgs, err
}
