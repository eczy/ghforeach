/*
 Copyright (c) 2024 Evan Czyzycki

 This program is free software: you can redistribute it and/or modify
 it under the terms of the GNU General Public License as published by
 the Free Software Foundation, either version 3 of the License, or
 (at your option) any later version.

 This program is distributed in the hope that it will be useful,
 but WITHOUT ANY WARRANTY; without even the implied warranty of
 MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 GNU General Public License for more details.

 You should have received a copy of the GNU General Public License
 along with this program. If not, see <https://www.gnu.org/licenses/>.
*/

package ghforeach_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/eczy/ghforeach/internal/ghforeach"
	"github.com/google/go-github/v60/github"
	"go.uber.org/zap"
)

type testHelper struct {
	client *github.Client
	org    string
	nRepos int

	repos []string
}

func newTestHelper(client *github.Client, org string, nRepos int) *testHelper {
	return &testHelper{client, org, nRepos, []string{}}
}

func (th *testHelper) getRepos() []string {
	return th.repos
}

func (th *testHelper) intToRepoName(i int) string {
	return fmt.Sprintf("ghforeach-test-%d", i)
}

func (th *testHelper) checkFileInRepo(ctx context.Context, repo, path string) (bool, error) {
	ghContent, _, resp, err := th.client.Repositories.GetContents(ctx, th.org, repo, path, nil)
	if resp.StatusCode == 404 {
		return false, nil
	} else if err != nil {
		return false, err
	}
	_, err = ghContent.GetContent()
	if err != nil {
		return false, err
	}
	return true, nil
}

func (th *testHelper) setup(ctx context.Context, t *testing.T) error {
	// create three test repositories
	for i := 0; i < th.nRepos; i++ {
		name := th.intToRepoName(i)
		_, _, err := th.client.Repositories.Create(ctx, th.org, &github.Repository{
			Name: github.String(name),
		})
		if err != nil {
			t.Logf("creating repo: %v", err)
		}
		_, _, err = th.client.Repositories.ReplaceAllTopics(ctx, th.org, name, []string{fmt.Sprintf("topic-%d", i)})
		if err != nil {
			t.Logf("replacing repo topics: %v", err)
		}
		_, _, err = th.client.Repositories.CreateFile(ctx, th.org, name, "README.md", &github.RepositoryContentFileOptions{
			Message: github.String("initial commit"),
			Content: []byte("test"),
		})
		if err != nil {
			t.Logf("creating README.md: %v", err)
		}
		th.repos = append(th.repos, name)
	}
	return nil
}

func (th *testHelper) teardown(ctx context.Context, t *testing.T) error {
	// tear down created test orgs
	for i := 0; i < th.nRepos; i++ {
		_, err := th.client.Repositories.Delete(ctx, th.org, th.intToRepoName(i))
		if err != nil {
			t.Logf("deleting repo: %v", err)
		}
	}
	return nil
}

func TestGhForeach_integration(t *testing.T) {
	cases := []struct {
		name          string
		command       string
		nameRegex     *string
		nameList      []string
		topicRegex    *string
		topicList     []string
		reposWithFile map[string]struct{}
	}{
		{
			"name regex only",
			"touch foobar.txt; git add .; git commit -m \"add foobar.txt\"; git push",
			github.String("ghforeach-test-[01]"),
			nil,
			nil,
			nil,
			map[string]struct{}{
				"ghforeach-test-0": {},
				"ghforeach-test-1": {},
			},
		},
		{
			"topic regex only",
			"touch foobar.txt; git add .; git commit -m \"add foobar.txt\"; git push",
			nil,
			nil,
			github.String("topic-[01]"),
			nil,
			map[string]struct{}{
				"ghforeach-test-0": {},
				"ghforeach-test-1": {},
			},
		},
		{
			"name list only",
			"touch foobar.txt; git add .; git commit -m \"add foobar.txt\"; git push",
			nil,
			[]string{
				"ghforeach-test-0",
				"ghforeach-test-1",
			},
			nil,
			nil,
			map[string]struct{}{
				"ghforeach-test-0": {},
				"ghforeach-test-1": {},
			},
		},
		{
			"topic list only",
			"touch foobar.txt; git add .; git commit -m \"add foobar.txt\"; git push",
			nil,
			nil,
			nil,
			[]string{
				"topic-0",
				"topic-1",
			},
			map[string]struct{}{
				"ghforeach-test-0": {},
				"ghforeach-test-1": {},
			},
		},
	}
	_, ok := os.LookupEnv("GHFOREACH_ENABLE_INTEGRATION_TEST")
	if !ok {
		t.Fatal("GHFOREACH_ENABLE_INTEGRATION_TEST not set")
	}
	user, ok := os.LookupEnv("GH_AUTH_USER")
	if !ok {
		t.Fatal("GH_AUTH_USER not set")
	}
	userToken, ok := os.LookupEnv("GH_AUTH_TOKEN")
	if !ok {
		t.Fatal("GH_AUTH_TOKEN not set")
	}
	testOrg, ok := os.LookupEnv("GHFOREACH_TEST_ORG")
	if !ok {
		t.Fatal("GHFOREACH_TEST_ORG not set")
	}
	client := github.NewClient(nil).WithAuthToken(userToken)
	logger, err := zap.NewDevelopment()
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			helper := newTestHelper(client, testOrg, 3)
			defer func() {
				t.Log("starting teardown")
				helper.teardown(ctx, t)
			}()
			t.Log("starting setup")
			err := helper.setup(ctx, t)
			if err != nil {
				t.Fatal(err)
			}

			opts := []ghforeach.RepositoryExecutorOption{
				ghforeach.WithClient(client),
				ghforeach.WithUserAuth(user, userToken),
				ghforeach.WithOrg(testOrg),
				ghforeach.WithCleanup(true),
				ghforeach.WithOverwrite(true),
				ghforeach.WithTmpDir("integ-tmp"),
				ghforeach.WithLogger(logger),
			}
			if tc.nameRegex != nil {
				opts = append(opts, ghforeach.WithNameRegexp(*tc.nameRegex))
			}
			if tc.nameList != nil {
				opts = append(opts, ghforeach.WithNameList(tc.nameList))
			}
			if tc.topicRegex != nil {
				opts = append(opts, ghforeach.WithTopicRegexp(*tc.topicRegex))
			}
			if tc.topicList != nil {
				opts = append(opts, ghforeach.WithTopicList(tc.topicList))
			}
			exec, err := ghforeach.NewRepositoryExecutor(opts...)
			if err != nil {
				t.Fatal(err)
			}
			err = exec.Go(ctx, tc.command)
			if err != nil {
				t.Fatal(err)
			}

			for _, repo := range helper.getRepos() {
				fileInRepo, err := helper.checkFileInRepo(ctx, repo, "foobar.txt")
				if err != nil {
					t.Fatal(err)
				}
				// should have file
				if _, ok := tc.reposWithFile[repo]; ok {
					if !fileInRepo {
						t.Errorf("file should be found in repo '%s'", repo)
					}
				} else {
					if fileInRepo {
						t.Errorf("file should not be found in repo '%s'", repo)
					}
				}
			}
		})
	}
}
