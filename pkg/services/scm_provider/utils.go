package scm_provider

import (
	"context"
	"fmt"
	"regexp"

	argoprojiov1alpha1 "github.com/argoproj/applicationset/api/v1alpha1"
)

func compileFilters(filters []argoprojiov1alpha1.SCMProviderGeneratorFilter) (Filters, error) {
	outFilters := make(Filters, 0, len(filters))
	for _, filter := range filters {
		outFilter := &Filter{}
		var err error
		if filter.RepositoryMatch != nil {
			outFilter.RepositoryMatch, err = regexp.Compile(*filter.RepositoryMatch)
			if err != nil {
				return nil, fmt.Errorf("error compiling RepositoryMatch regexp %q: %v", *filter.RepositoryMatch, err)
			}
		}
		if filter.LabelMatch != nil {
			outFilter.LabelMatch, err = regexp.Compile(*filter.LabelMatch)
			if err != nil {
				return nil, fmt.Errorf("error compiling LabelMatch regexp %q: %v", *filter.LabelMatch, err)
			}
		}
		if filter.PathsExist != nil {
			outFilter.PathsExist = filter.PathsExist
		}
		if filter.BranchMatch != nil {
			outFilter.BranchMatch, err = regexp.Compile(*filter.BranchMatch)
			if err != nil {
				return nil, fmt.Errorf("error compiling BranchMatch regexp %q: %v", *filter.LabelMatch, err)
			}
		}
		outFilters = append(outFilters, outFilter)
	}
	return outFilters, nil
}

func matchFilter(ctx context.Context, provider SCMProviderService, repo *Repository, filter *Filter) (bool, error) {
	//fmt.Printf("Oleg: repositoryMatch: %v, branchMatch: %v, repoName: %v, branchName: %v", filter.RepositoryMatch.String(), filter.BranchMatch.String(), repo.Repository, repo.Branch)
	if filter.RepositoryMatch != nil && !filter.RepositoryMatch.MatchString(repo.Repository) {
		fmt.Println("Oleg: No repository match")
		return false, nil
	}

	if filter.BranchMatch != nil && !filter.BranchMatch.MatchString(repo.Branch) {
		fmt.Println("Oleg: No branch match")
		return false, nil
	}

	if filter.LabelMatch != nil {
		fmt.Println("Oleg: Label filter isn't empty")
		found := false
		for _, label := range repo.Labels {
			if filter.LabelMatch.MatchString(label) {
				found = true
				break
			}
		}
		if !found {
			return false, nil
		}
	}

	if len(filter.PathsExist) != 0 {
		fmt.Println("Oleg: Path filter isn't empty")
		for _, path := range filter.PathsExist {
			hasPath, err := provider.RepoHasPath(ctx, repo, path)
			if err != nil {
				return false, err
			}
			if !hasPath {
				return false, nil
			}
		}
	}

	return true, nil
}

func ListRepos(ctx context.Context, provider SCMProviderService, filters []argoprojiov1alpha1.SCMProviderGeneratorFilter, cloneProtocol string) ([]*Repository, error) {
	fmt.Println("Oleg: 1")
	compiledFilters, err := compileFilters(filters)
	if err != nil {
		return nil, err
	}

	fmt.Println("Oleg: 2")
	repos, err := provider.ListRepos(ctx, cloneProtocol)
	if err != nil {
		fmt.Println("Oleg: 7. got strange error")
		return nil, err
	}

	fmt.Println("Oleg: 3")
	repoFilters := compiledFilters.GetRepoFilters()
	fmt.Printf("Oleg: length of repoFilters: %v\n", len(repoFilters))
	fmt.Printf("Oleg: repoFilters content: %v\n", repoFilters)
	filteredRepos := make([]*Repository, 0, len(repos))
	if len(repoFilters) == 0 {
		fmt.Println("Oleg: Empty repoFilters")
		filteredRepos = repos
	} else {
		fmt.Println("Oleg: Will start iterating repos in ListRepos")
		for _, repo := range repos {
			fmt.Println("Oleg: Iterating repos in ListRepos")
			for _, filter := range repoFilters {
				fmt.Println("Oleg: checking repoFilters")
				matches, err := matchFilter(ctx, provider, repo, filter)
				if err != nil {
					return nil, err
				}
				fmt.Println("Oleg: checking filteredRepos match")
				if matches {
					fmt.Println("Oleg: appending filteredRepos")
					filteredRepos = append(filteredRepos, repo)
					break
				}
			}
		}
	}

	repos, err = getBranches(ctx, provider, filteredRepos, compiledFilters)
	if err != nil {
		return nil, err
	}
	return repos, nil
}

func getBranches(ctx context.Context, provider SCMProviderService, repos []*Repository, compiledFilters Filters) ([]*Repository, error) {
	fmt.Println("Oleg: I'm in getBranches func")
	reposWithBranches := []*Repository{}
	for _, repo := range repos {
		fmt.Printf("Oleg: 111 %v %v", repo.Branch, repo.Repository)
		reposFilled, err := provider.GetBranches(ctx, repo)
		if err != nil {
			fmt.Println("Oleg: GetBranches returned error")
			return nil, err
		}

		fmt.Println("Oleg: Appending reposWithBranches")
		reposWithBranches = append(reposWithBranches, reposFilled...)
	}
	branchFilters := compiledFilters.GetBranchFilters()

	if len(branchFilters) == 0 {
		fmt.Println("Oleg: branchFilters are not set")
		return reposWithBranches, nil
	}
	filteredRepos := make([]*Repository, 0, len(reposWithBranches))
	for _, repo := range reposWithBranches {
		fmt.Println("Oleg: Iterating over reposWithBranches")
		for _, filter := range branchFilters {
			fmt.Println("Oleg: matching Branch")
			matches, err := matchFilter(ctx, provider, repo, filter)
			if err != nil {
				return nil, err
			}
			if matches {
				fmt.Println("Oleg: branch match found")
				filteredRepos = append(filteredRepos, repo)
				break
			}
		}
	}
	return filteredRepos, nil
}
