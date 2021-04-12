package datasource

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

func GithubGet() {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: os.Getenv("GITHUB_ACCESS_TOKEN")},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	// list all Organizations for the authenticated user
	orgs, _, _ := client.Organizations.List(ctx, "", nil)
	fmt.Printf("---ORGS %d \n", len(orgs))

	orgWhitelist := strings.Split(os.Getenv("ORG_WHITELIST"), ",")
	orgBlacklist := strings.Split(os.Getenv("ORG_BLACKLIST"), ",")

	//orgNameList := []string{}
	for i := range orgs {
		if orgs[i].Login != nil {
			orgName := *orgs[i].Login
			if listContains(orgBlacklist, orgName) {
				continue
			}
			if len(orgWhitelist) == 0 || listContains(orgWhitelist, orgName) {
				repos, _ := GetAllReposForOrg(orgName)
				for i, r := range repos {
					fmt.Printf("%d %s -- %s\n", i, orgName, *r.Name)
					/*
						prs, _, err := client.PullRequests.List(ctx, orgName, *r.Name, nil)
						if err != nil {
							fmt.Printf("%s", err)
						} else {
							fmt.Printf("%s -- %s prs: %d\n", orgName, *r.Name, len(prs))
						}
					*/
				}
			}
		}
	}

	//prs, _, err := client.PullRequests.List(ctx, "hellodigit", "digit-libs", nil)
	//fmt.Printf("NUM prs %d \n", len(prs))
	//fmt.Printf("err %s \n", err)
	/*
		for i := range prs {
			fmt.Printf("%+v \n", prs[i])
		}
	*/
}

func listContains(list []string, val string) bool {
	for _, s := range list {
		if s == val {
			return true
		}
	}
	return false
}
