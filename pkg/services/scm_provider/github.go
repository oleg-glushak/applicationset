package scm_provider

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/google/go-github/v35/github"
	"golang.org/x/oauth2"
)

type GithubProvider struct {
	client       *github.Client
	organization string
	allBranches  bool
}

var _ SCMProviderService = &GithubProvider{}

func NewGithubProvider(ctx context.Context, organization string, token string, url string, allBranches bool) (*GithubProvider, error) {
	var ts oauth2.TokenSource
	// Undocumented environment variable to set a default token, to be used in testing to dodge anonymous rate limits.
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}
	if token != "" {
		ts = oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: token},
		)
	}
	httpClient := oauth2.NewClient(ctx, ts)
	var client *github.Client
	if url == "" {
		client = github.NewClient(httpClient)
	} else {
		var err error
		client, err = github.NewEnterpriseClient(url, url, httpClient)
		if err != nil {
			return nil, err
		}
	}
	return &GithubProvider{client: client, organization: organization, allBranches: allBranches}, nil
}

func (g *GithubProvider) GetBranches(ctx context.Context, repo *Repository) ([]*Repository, error) {
	repos := []*Repository{}
	fmt.Println("Oleg: I'm in github GetBranches")
	branches, err := g.listBranches(ctx, repo)
	if err != nil {
		return nil, fmt.Errorf("error listing branches for %s/%s: %v", repo.Organization, repo.Repository, err)
	}

	re, err := regexp.Compile(`[^\w]`)
	if err != nil {
		return nil, fmt.Errorf("error listing branches for %s/%s: %v", repo.Organization, repo.Repository, err)
	}

	for _, branch := range branches {
		fmt.Printf("Oleg: appending branch in Github GetBranches %v", branch.GetName())
		longSHA := branch.GetCommit().GetSHA()
		var shortSHA string
		if len(longSHA) >= 7 {
			shortSHA = longSHA[0:6]
		} else {
			shortSHA = longSHA
		}
		repos = append(repos, &Repository{
			Organization: repo.Organization,
			Repository:   repo.Repository,
			URL:          repo.URL,
			Branch:       branch.GetName(),
			// normalise Git branch name and make it unique for dynamic environment deployments
			SHA:          strings.ToLower(re.ReplaceAllString(branch.GetName(), "")) + "-" + shortSHA,
			Labels:       repo.Labels,
			RepositoryId: repo.RepositoryId,
		})
	}
	return repos, nil
}

func (g *GithubProvider) ListRepos(ctx context.Context, cloneProtocol string) ([]*Repository, error) {
	opt := &github.RepositoryListByOrgOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}
	repos := []*Repository{}
	for {
		fmt.Println("Oleg: I'm in github ListRepos")
		githubRepos, resp, err := g.client.Repositories.ListByOrg(ctx, g.organization, opt)
		if err != nil {
			return nil, fmt.Errorf("error listing repositories for %s: %v", g.organization, err)
		}
		fmt.Println("Oleg: 4")
		for _, githubRepo := range githubRepos {
			fmt.Println("Oleg: 5")
			var url string
			switch cloneProtocol {
			// Default to SSH if unspecified (i.e. if "").
			case "", "ssh":
				url = githubRepo.GetSSHURL()
			case "https":
				url = githubRepo.GetCloneURL()
			default:
				return nil, fmt.Errorf("unknown clone protocol for GitHub %v", cloneProtocol)
			}
			fmt.Println("Oleg: appending 6")
			repos = append(repos, &Repository{
				Organization: githubRepo.Owner.GetLogin(),
				Repository:   githubRepo.GetName(),
				Branch:       githubRepo.GetDefaultBranch(),
				URL:          url,
				Labels:       githubRepo.Topics,
				RepositoryId: githubRepo.ID,
			})
		}
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	return repos, nil
}

func (g *GithubProvider) RepoHasPath(ctx context.Context, repo *Repository, path string) (bool, error) {
	_, _, resp, err := g.client.Repositories.GetContents(ctx, repo.Organization, repo.Repository, path, &github.RepositoryContentGetOptions{
		Ref: repo.Branch,
	})
	// 404s are not an error here, just a normal false.
	if resp != nil && resp.StatusCode == 404 {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (g *GithubProvider) listBranches(ctx context.Context, repo *Repository) ([]github.Branch, error) {
	// If we don't specifically want to query for all branches, just use the default branch and call it a day.

	fmt.Println("Oleg: I'm in github listBranches")
	if !g.allBranches {
		fmt.Println("Oleg: allBranches is set to false")
		defaultBranch, _, err := g.client.Repositories.GetBranch(ctx, repo.Organization, repo.Repository, repo.Branch)
		if err != nil {
			var githubErrorResponse *github.ErrorResponse
			if errors.As(err, &githubErrorResponse) {
				if githubErrorResponse.Response.StatusCode == 404 {
					// Default branch doesn't exist, so the repo is empty.
					return []github.Branch{}, nil
				}
			}
			return nil, err
		}
		return []github.Branch{*defaultBranch}, nil
	}
	// Otherwise, scrape the ListBranches API.
	fmt.Println("Oleg: allBranches is set to true")
	opt := &github.BranchListOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}
	branches := []github.Branch{}
	for {
		githubBranches, resp, err := g.client.Repositories.ListBranches(ctx, repo.Organization, repo.Repository, opt)
		if err != nil {
			return nil, err
		}
		for _, githubBranch := range githubBranches {
			fmt.Printf("Oleg: appending branch %v", githubBranch.GetName())
			branches = append(branches, *githubBranch)
		}

		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	return branches, nil
}
