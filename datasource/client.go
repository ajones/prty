package datasource

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

type Datasource struct {
	statusChan chan<- string

	myUsername      string
	myTeamUsernames []string
}

var sharedGithubClient *github.Client

func sharedClient() *github.Client {
	if sharedGithubClient == nil {
		ctx := context.Background()
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: os.Getenv("GITHUB_ACCESS_TOKEN")},
		)
		tc := oauth2.NewClient(ctx, ts)

		sharedGithubClient = github.NewClient(tc)
	}

	return sharedGithubClient
}

func New() *Datasource {
	ds := &Datasource{}
	ds.myUsername = os.Getenv("GITHUB_USERNAME")
	ds.myTeamUsernames = strings.Split(os.Getenv("GITHUB_USERNAME"), ",")
	return ds
}

func (d *Datasource) writeStatus(message string) {
	d.statusChan <- message
}

func (d *Datasource) SetStatusChan(statusChan chan<- string) {
	d.statusChan = statusChan
}

func (d *Datasource) RefreshData() {
	d.writeStatus("refreshing data...")

	orgWhitelist := strings.Split(os.Getenv("ORG_WHITELIST"), ",")
	orgBlacklist := strings.Split(os.Getenv("ORG_BLACKLIST"), ",")

	d.writeStatus("fetching orgs...")
	ctx := context.Background()
	orgs, _, _ := sharedClient().Organizations.List(ctx, "", nil)

	for i := range orgs {
		if orgs[i].Login != nil {
			orgName := *orgs[i].Login
			if listContains(orgBlacklist, orgName) {
				continue
			}
			if len(orgWhitelist) == 0 || listContains(orgWhitelist, orgName) {
				d.writeStatus(fmt.Sprintf("%s fetching repos...", orgName))

				repos, _ := GetAllReposForOrg(orgName)
				for _, repo := range repos {
					repoName := *repo.Name
					d.writeStatus(fmt.Sprintf("%s/%s fetching prs...", orgName, repoName))
					prs, _, err := sharedClient().PullRequests.List(ctx, orgName, repoName, nil)
					d.OnUpdatedPulls(orgName, repoName, prs)

					if err != nil {
						fmt.Printf("%s", err)
						d.writeStatus(fmt.Sprintf("ERR: %s", err))
					}
				}
			}
		}
	}
	d.writeStatus("update complete")
}
