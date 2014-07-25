// Copyright 2014 The Azul3D Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"github.com/google/go-github/github"
	"os"
	"path/filepath"
	"strings"
)

var (
	ghOrganization = "azul3d"
	baseImport     = "azul3d.org/"
	ignoredRepos   = map[string]bool{
		"azul3d.github.io": true,
		"issues":           true,
	}

	API_TOKEN = os.Getenv("GITHUB_API_TOKEN")
	ghClient  *github.Client
)

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
		ListOptions: github.ListOptions{PerPage: 10},
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
	opt := &github.ListOptions{PerPage: 10}
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

type repo struct {
	github.Repository
	Tags []github.RepositoryTag
}

func fetchRepos(ghClient *github.Client) (map[string]repo, error) {
	rs, err := fetchAllRepos(ghClient)
	if err != nil {
		return nil, err
	}
	repos := make(map[string]repo, len(rs))
	for _, r := range rs {
		tags, err := fetchTags(ghClient, *r.Name)
		if err != nil {
			return nil, err
		}
		repos[*r.Name] = repo{
			Repository: r,
			Tags:       tags,
		}
	}
	return repos, nil
}
