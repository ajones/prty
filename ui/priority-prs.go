package ui

import (
	"sort"

	"github.com/inburst/prty/datasource"
)

/*
Got data
- PR author
- requested reviewers / Are there people tagged in the PR


- number of comments and comment authors
- Time since last comment
- has there been changes since last comment
- have I made comments on this PR
- total age of PR
*/

type PriorityPRs struct {
	PRView
}

func (p *PriorityPRs) OnNewPullData(pr *datasource.PullRequest) {
	if !pr.IsAbandoned && !pr.IsDraft && !pr.IsApproved && !pr.AuthorIsBot && pr.HasChangesAfterLastComment {
		p.pulls = append(p.pulls, pr)
		p.needsSort = true
	}
}

func (p *PriorityPRs) OnSort() {
	sort.Sort(byImportance(p.pulls))
	p.needsSort = false
}
