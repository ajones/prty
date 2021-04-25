package datasource

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-github/v34/github"
	"golang.org/x/oauth2"
)

func CheckAccessToken(tok string, userName string) error {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: tok},
	)
	tc := oauth2.NewClient(ctx, ts)
	sharedGithubClient = github.NewClient(tc)
	_, resp, err := sharedClient().Users.Get(ctx, userName)
	if err != nil {
		return err
	}
	scopesStr := resp.Header.Get("X-OAuth-Scopes")
	scopes := strings.Split(scopesStr, ",")
	expectedScopes := []string{
		"repo",
		"read:org",
	}
	missing := []string{}
	for _, es := range expectedScopes {
		found := false
		for _, s := range scopes {
			trimmed := strings.Trim(s, " ")
			if trimmed == es {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, es)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("Required scopes are [repo, org:read]. Did not find [%s].\nPlease re-issue a new token with the required scopes.", strings.Join(missing, ","))
}
