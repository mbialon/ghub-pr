package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/manifoldco/promptui"
	"github.com/shurcooL/githubv4"
	"golang.org/x/oauth2"
)

type PullRequest struct {
	Number      int
	Title       string
	BaseRefName string
	HeadRefName string
	Closed      bool
}

func main() {
	owner := flag.String("owner", "", "github repository owner")
	name := flag.String("name", "", "github repository name")
	command := flag.String("exec", "", "exec command")
	all := flag.Bool("all", false, "list both open and closed")
	flag.Parse()

	src := oauth2.StaticTokenSource(
		&oauth2.Token{
			AccessToken: os.Getenv("GITHUB_TOKEN"),
		},
	)
	httpClient := oauth2.NewClient(context.Background(), src)
	client := githubv4.NewClient(httpClient)

	var q struct {
		Repository struct {
			PullRequests struct {
				Nodes    []PullRequest
				PageInfo struct {
					EndCursor   githubv4.String
					HasNextPage bool
				}
			} `graphql:"pullRequests(first: 100, after: $pullRequestsCursor)"`
		} `graphql:"repository(owner: $repositoryOwner, name: $repositoryName)"`
	}
	vars := map[string]interface{}{
		"repositoryOwner":    githubv4.String(*owner),
		"repositoryName":     githubv4.String(*name),
		"pullRequestsCursor": (*githubv4.String)(nil),
	}
	var prs []PullRequest
	for {
		err := client.Query(context.Background(), &q, vars)
		if err != nil {
			log.Fatal(err)
		}
		for _, pr := range q.Repository.PullRequests.Nodes {
			if pr.Closed && !*all {
				continue
			}
			prs = append(prs, pr)
		}
		if !q.Repository.PullRequests.PageInfo.HasNextPage {
			break
		}
		vars["pullRequestsCursor"] = githubv4.NewString(q.Repository.PullRequests.PageInfo.EndCursor)
	}

	templates := &promptui.SelectTemplates{
		Active:   "> {{ .Title }}",
		Inactive: "  {{ .Title }}",
		Selected: "{{ .Number }} {{ .HeadRefName }}",
	}

	searcher := func(input string, index int) bool {
		pr := prs[index]
		name := strings.Replace(strings.ToLower(pr.Title), " ", "", -1)
		input = strings.Replace(strings.ToLower(input), " ", "", -1)

		return strings.Contains(name, input)
	}

	s := promptui.Select{
		Label:     "Select a pull-request",
		Items:     prs,
		Templates: templates,
		Size:      4,
		Searcher:  searcher,
	}
	i, _, err := s.Run()
	if err != nil {
		log.Fatal(err)
	}
	cmd := exec.Command(*command, strconv.Itoa(prs[i].Number), prs[i].HeadRefName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatal(err)
	}
}
