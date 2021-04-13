package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/google/go-github/v34/github"
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
	fmt.Printf("NUM ORGS %d \n", len(orgs))
	for i := range orgs {
		fmt.Printf("%+v \n", orgs[i])

		if orgs[i].Login != nil {
			fmt.Printf("%+v \n", *orgs[i].Login)
		}
	}

	prs, _, err := client.PullRequests.List(ctx, "hellodigit", "digit-libs", nil)
	fmt.Printf("NUM prs %d \n", len(prs))
	fmt.Printf("err %s \n", err)
	for i := range prs {
		fmt.Printf("%+v \n", prs[i])

		/*
			if orgs[i].Login != nil {
				fmt.Printf("%+v \n", *orgs[i].Login)
			}
		*/
	}
}
