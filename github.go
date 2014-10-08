// Copyright 2014 The Azul3D Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"os"
	"path/filepath"
	"strings"

	"code.google.com/p/goauth2/oauth"
	"github.com/google/go-github/github"
)

var (
	ghOrganization = "azul3d"
	baseImport     = "azul3d.org/"
	ignoredRepos   = map[string]bool{
		"azul3d.github.io": true,
		"appengine":        true,
		"issues":           true,
		"examples":         true,
		"cmd-webgen":       true,
		"cmd-azulfix":      true,

		// Deprecated packages.
		"thirdparty-resize": true,
		"cmd-glwrap":        true,
		"native-gl":         true,
		"native-gles2":      true,
		"chippy":            true,
	}

	API_TOKEN = os.Getenv("GITHUB_API_TOKEN")
	ghClients chan *github.Client
)

func ghInitClients(token string, n int) {
	ghClients = make(chan *github.Client, n)
	for i := 0; i < n; i++ {
		if len(token) > 0 {
			t := &oauth.Transport{
				Token: &oauth.Token{AccessToken: API_TOKEN},
			}
			ghClients <- github.NewClient(t.Client())
			continue
		}
		ghClients <- github.NewClient(nil)
	}
}

func dashToSlash(dashes string) string {
	return strings.Replace(dashes, "-", "/", -1)
}

func importURL(repoName, versionTag string) string {
	// dashes in repository names are replaced by slashes, so the github
	// repository at:
	//  github.com/azul3d/native-freetype
	// would be import as:
	//  azul3d.org/native/freetype.v1
	repoName = strings.Replace(repoName, "-", "/", -1)
	return filepath.Join(baseImport, repoName) + "." + versionTag
}

func fetchAllRepos(ghClient *github.Client) ([]github.Repository, error) {
	opt := &github.RepositoryListByOrgOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}
	var allRepos []github.Repository
	for {
		repos, resp, err := ghClient.Repositories.ListByOrg(ghOrganization, opt)
		if err != nil {
			return nil, err
		}
		allRepos = append(allRepos, repos...)
		if resp.NextPage == 0 {
			break
		}
		opt.ListOptions.Page = resp.NextPage
	}
	return allRepos, nil
}

func fetchTags(ghClient *github.Client, repo string) ([]github.RepositoryTag, error) {
	opt := &github.ListOptions{PerPage: 100}
	var allTags []github.RepositoryTag
	for {
		tags, resp, err := ghClient.Repositories.ListTags(ghOrganization, repo, opt)
		if err != nil {
			return nil, err
		}
		allTags = append(allTags, tags...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	return allTags, nil
}

func fetchBranches(ghClient *github.Client, repo string) ([]github.Branch, error) {
	opt := &github.ListOptions{PerPage: 100}
	var allBranches []github.Branch
	for {
		branches, resp, err := ghClient.Repositories.ListBranches(ghOrganization, repo, opt)
		if err != nil {
			return nil, err
		}
		allBranches = append(allBranches, branches...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	return allBranches, nil
}

type repo struct {
	github.Repository
	Tags     []github.RepositoryTag
	Branches []github.Branch
}

func fetchRepos() (map[string]repo, error) {
	client := <-ghClients
	rs, err := fetchAllRepos(client)
	ghClients <- client
	if err != nil {
		return nil, err
	}

	// Fetch tags and branches using multiple goroutines.
	var (
		errors    = make(chan error, 16)
		reposChan = make(chan repo, 16)
		firstErr  error
	)
	for _, toCpy := range rs {
		r := toCpy
		go func() {
			client := <-ghClients
			tags, tagsErr := fetchTags(client, *r.Name)
			branches, branchesErr := fetchBranches(client, *r.Name)
			ghClients <- client
			if tagsErr != nil {
				errors <- tagsErr
				return
			}
			if branchesErr != nil {
				errors <- branchesErr
				return
			}
			reposChan <- repo{
				Repository: r,
				Tags:       tags,
				Branches:   branches,
			}
		}()
	}

	// Wait for all goroutines to finish, either with a result or an error.
	repos := make(map[string]repo, len(rs))
	done := 0
	for done < len(rs) {
		select {
		case r := <-reposChan:
			repos[*r.Name] = r
			done++
		case err = <-errors:
			done++
			if firstErr == nil {
				firstErr = err
			}
		}
	}

	// Return the first error that occured, if any.
	if firstErr != nil {
		return nil, firstErr
	}
	return repos, nil
}
