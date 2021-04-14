package datasource

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/google/go-github/v34/github"
	"golang.org/x/oauth2"
)

type Datasource struct {
	statusChan            chan<- string
	prUpdateChan          chan<- *PullRequest
	remainingRequestsChan chan<- string

	myUsername      string
	myTeamUsernames []string

	orgWhitelist []string
	orgBlacklist []string
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

	ds.orgWhitelist = strings.Split(os.Getenv("ORG_WHITELIST"), ",")
	ds.orgBlacklist = strings.Split(os.Getenv("ORG_BLACKLIST"), ",")

	return ds
}

func (ds *Datasource) writeErrorStatus(err error) {
	ds.statusChan <- fmt.Sprintf("ERROR: %s", err)
}

func (ds *Datasource) writeStatus(message string) {
	ds.statusChan <- message
}

func (ds *Datasource) SetStatusChan(statusChan chan<- string) {
	ds.statusChan = statusChan
}

func (ds *Datasource) SetRemainingRequestsChan(remainingRequestsChan chan<- string) {
	ds.remainingRequestsChan = remainingRequestsChan
}

func (ds *Datasource) SetPRUpdateChan(prUpdateChan chan<- *PullRequest) {
	ds.prUpdateChan = prUpdateChan
}

func (ds *Datasource) RefreshData() {
	ds.writeStatus("fetching orgs...")

	orgs, err := GetAllOrgs()
	if err != nil {
		ds.writeErrorStatus(err)
		return
	}
	ds.writeStatus("gotem ")

	for i := range orgs {
		if orgs[i].Login != nil {
			orgName := *orgs[i].Login
			if listContains(ds.orgBlacklist, orgName) {
				continue
			}
			// if the whitelist is empty or this oprg is whitelisted
			if len(ds.orgWhitelist) == 0 || listContains(ds.orgWhitelist, orgName) {

				ds.writeStatus(fmt.Sprintf("%s fetching repos...", orgName))
				repos, err := GetAllReposForOrg(orgName)
				if err != nil {
					ds.writeErrorStatus(err)
					println(fmt.Sprintf("%s\n", err))
					return
				}

				for _, repo := range repos {
					repoName := *repo.Name

					ds.writeStatus(fmt.Sprintf("%s/%s fetching prs...", orgName, repoName))
					prs, err := GetAllPullsForRepoInOrg(orgName, repoName)
					if err != nil {
						ds.writeErrorStatus(err)
						println(fmt.Sprintf("%s\n", err))
						return
					}

					for _, ghpr := range prs {
						newPR, err := ds.BuildPullRequest(orgName, repoName, ghpr)
						if err != nil {
							ds.writeErrorStatus(err)
							println(fmt.Sprintf("%s\n", err))
							continue
						}
						ds.prUpdateChan <- newPR
					}
				}
			}
		}
	}
	ds.writeStatus("update complete")
}
