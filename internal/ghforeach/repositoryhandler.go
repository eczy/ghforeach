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

package ghforeach

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"regexp"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/google/go-github/v60/github"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type executionResult struct {
	Path    string `json:"path"`
	Command string `json:"command"`
	Stdout  string `json:"stdout"`
	Stderr  string `json:"stderr"`
	Error   error  `json:"error"`
}

func (er *executionResult) String() string {
	str := fmt.Sprintf(">>>>> %s: %s\n", er.Path, er.Command)
	str += fmt.Sprintf("STDERR:\n%s\n", er.Stderr)
	str += fmt.Sprintf("STDOUT:\n%s\n", er.Stdout)
	if er.Error != nil {
		str += fmt.Sprintf("error: %v\n", er.Error)
	}
	return str
}

func (er *executionResult) JsonString() (string, error) {
	str, err := json.Marshal(*er)
	if err != nil {
		return "", err
	}
	return string(str), err
}

type RepositoryExecutorOutputFormat = int

const (
	ConsoleOutputFormat RepositoryExecutorOutputFormat = iota
	JsonOutputFormat
)

type RepositoryExecutorOption = func(*RepositoryExecutor) error

func WithOrg(org string) RepositoryExecutorOption {
	return func(fre *RepositoryExecutor) error {
		fre.org = &org
		return nil
	}
}

func WithUser(user string) RepositoryExecutorOption {
	return func(fre *RepositoryExecutor) error {
		fre.user = &user
		return nil
	}
}

func WithNameRegexp(exp string) RepositoryExecutorOption {
	return func(fre *RepositoryExecutor) error {
		regexp, err := regexp.Compile(exp)
		if err != nil {
			return err
		}
		fre.nameRegexp = regexp
		return nil
	}
}

func WithNameList(names []string) RepositoryExecutorOption {
	return func(fre *RepositoryExecutor) error {
		fre.nameSet = map[string]struct{}{}
		for _, name := range names {
			fre.nameSet[name] = struct{}{}
		}
		return nil
	}
}

func WithTopicRegexp(exp string) RepositoryExecutorOption {
	return func(fre *RepositoryExecutor) error {
		regexp, err := regexp.Compile(exp)
		if err != nil {
			return err
		}
		fre.topicRegexp = regexp
		return nil
	}
}

func WithTopicList(topics []string) RepositoryExecutorOption {
	return func(fre *RepositoryExecutor) error {
		fre.topicSet = map[string]struct{}{}
		for _, topic := range topics {
			fre.topicSet[topic] = struct{}{}
		}
		return nil
	}
}

func WithClient(client *github.Client) RepositoryExecutorOption {
	return func(fre *RepositoryExecutor) error {
		fre.client = client
		return nil
	}
}

func WithLogger(logger *zap.Logger) RepositoryExecutorOption {
	return func(fre *RepositoryExecutor) error {
		fre.logger = logger
		return nil
	}
}

func WithUserAuth(user, token string) RepositoryExecutorOption {
	return func(fre *RepositoryExecutor) error {
		fre.authUser = &user
		fre.authToken = &token
		return nil
	}
}

func WithOverwrite(b bool) RepositoryExecutorOption {
	return func(fre *RepositoryExecutor) error {
		fre.overwrite = b
		return nil
	}
}

func WithCleanup(b bool) RepositoryExecutorOption {
	return func(fre *RepositoryExecutor) error {
		fre.cleanup = b
		return nil
	}
}

func WithTmpDir(dir string) RepositoryExecutorOption {
	return func(fre *RepositoryExecutor) error {
		fre.tmpDir = dir
		return nil
	}
}

func WithConcurrency(n int) RepositoryExecutorOption {
	return func(fre *RepositoryExecutor) error {
		fre.concurrency = n
		return nil
	}
}

func WithOutputFormat(format RepositoryExecutorOutputFormat) RepositoryExecutorOption {
	return func(fre *RepositoryExecutor) error {
		fre.outputFormat = format
		return nil
	}
}

func WithShellPath(path string) RepositoryExecutorOption {
	return func(fre *RepositoryExecutor) error {
		fre.shellPath = path
		return nil
	}
}

type RepositoryExecutor struct {
	// api parameters
	client    *github.Client
	logger    *zap.Logger
	authUser  *string
	authToken *string

	// filter parameters
	nameRegexp  *regexp.Regexp
	nameSet     map[string]struct{}
	topicRegexp *regexp.Regexp
	topicSet    map[string]struct{}

	// operation parameters
	shellPath    string
	overwrite    bool
	cleanup      bool
	tmpDir       string
	concurrency  int
	outputFormat RepositoryExecutorOutputFormat
	org          *string
	user         *string
}

func NewRepositoryExecutor(opts ...RepositoryExecutorOption) (*RepositoryExecutor, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	exec := &RepositoryExecutor{
		client:      github.NewClient(nil),
		logger:      zap.L(),
		tmpDir:      path.Join(wd, "tmp"),
		concurrency: 1,
		shellPath:   "/bin/sh",
	}

	for _, opt := range opts {
		err := opt(exec)
		if err != nil {
			return nil, err
		}
	}

	return exec, nil
}

func (rh *RepositoryExecutor) Go(ctx context.Context, command string) error {
	if rh.overwrite {
		rh.logger.Debug("removing temp directory", zap.String("path", rh.tmpDir))
		err := os.RemoveAll(rh.tmpDir)
		if err != nil {
			return err
		}
	}

	if _, err := os.Stat(rh.tmpDir); errors.Is(err, os.ErrNotExist) {
		cd, _ := os.Getwd()
		rh.logger.Debug("creating temp directory", zap.String("path", rh.tmpDir), zap.String("cd", cd))
		err := os.MkdirAll(rh.tmpDir, 0700)
		if err != nil {
			return err
		}
	}

	if rh.cleanup {
		defer func() {
			rh.logger.Debug("removing temp directory", zap.String("path", rh.tmpDir))
			os.RemoveAll(rh.tmpDir)
		}()
	}

	g, ctx := errgroup.WithContext(ctx)
	repoCh := make(chan *github.Repository)
	resultCh := make(chan *executionResult)

	g.Go(func() error {
		defer close(repoCh)
		return rh.getRepositories(ctx, repoCh)
	})

	g.Go(func() error {
		defer close(resultCh)
		repoG, repoCtx := errgroup.WithContext(ctx)
		repoG.SetLimit(rh.concurrency)
		for repo := range repoCh {
			select {
			case <-ctx.Done():
				rh.logger.Error("context error", zap.Error(ctx.Err()))
			default:
				repoG.Go(func() error {
					repoDir := path.Join(rh.tmpDir, repo.GetName())
					if _, err := os.Stat(repoDir); errors.Is(err, os.ErrNotExist) {
						err := rh.cloneRepo(repoCtx, repoDir, repo)
						if err != nil {
							rh.logger.Error("error cloning repository", zap.String("repository", repo.GetName()), zap.Error(err))
							return nil
						}
					}

					if rh.cleanup {
						defer func() {
							os.RemoveAll(repoDir)
						}()
					}

					stdoutBuf := &bytes.Buffer{}
					stderrBuf := &bytes.Buffer{}
					err := rh.execCommand(command, repoDir, stdoutBuf, stderrBuf)
					if err != nil {
						rh.logger.Error("error executing command", zap.String("command", command), zap.Error(err))
					}
					resultCh <- &executionResult{
						Path:    repoDir,
						Command: command,
						Stdout:  stdoutBuf.String(),
						Stderr:  stderrBuf.String(),
						Error:   err,
					}
					return nil
				})
			}
		}
		return repoG.Wait()
	})

	g.Go(func() error {
		for result := range resultCh {
			select {
			case <-ctx.Done():
				rh.logger.Error("context error", zap.Error(ctx.Err()))
			default:
				switch rh.outputFormat {
				case JsonOutputFormat:
					str, err := result.JsonString()
					if err != nil {
						rh.logger.Error("error marshalling result to json", zap.Error(err))
					} else {
						fmt.Println(str)
					}
				case ConsoleOutputFormat:
					fmt.Println(result.String())
				default:
					rh.logger.Error("invalid output format", zap.Any("format", rh.outputFormat))
				}
			}
		}
		return nil
	})

	return g.Wait()
}

func (rh *RepositoryExecutor) getRepositories(ctx context.Context, ch chan<- *github.Repository) error {
	if rh.org != nil {
		rh.logger.Debug("fetching organization repositories", zap.String("organization", *rh.org))
		return rh.getRepositoriesForOrg(ctx, *rh.org, ch)
	} else if rh.authUser != nil && rh.authToken != nil && rh.user != nil && *rh.authUser == *rh.user {
		rh.logger.Debug("fetching user repositories", zap.String("user", *rh.user))
		return rh.getRepositoriesForAuthenticatedUser(ctx, ch)
	} else if rh.user != nil {
		rh.logger.Debug("fetching user repositories", zap.String("user", *rh.user))
		return rh.getRepositoriesForUser(ctx, *rh.user, ch)
	} else {
		return fmt.Errorf("no user or org specified")
	}
}

func (rh *RepositoryExecutor) getRepositoriesForOrg(ctx context.Context, org string, ch chan<- *github.Repository) error {
	opt := &github.RepositoryListByOrgOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		repos, resp, err := rh.client.Repositories.ListByOrg(ctx, org, opt)
		if err != nil {
			return err
		}
		for _, repo := range repos {
			if rh.matchRepo(repo) {
				ch <- repo
			}
		}
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	return nil
}

func (rh *RepositoryExecutor) getRepositoriesForUser(ctx context.Context, user string, ch chan<- *github.Repository) error {
	opt := &github.RepositoryListByUserOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		repos, resp, err := rh.client.Repositories.ListByUser(ctx, user, opt)
		if err != nil {
			return err
		}
		for _, repo := range repos {
			if rh.matchRepo(repo) {
				ch <- repo
			}
		}
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	return nil
}

func (rh *RepositoryExecutor) getRepositoriesForAuthenticatedUser(ctx context.Context, ch chan<- *github.Repository) error {
	opt := &github.RepositoryListByAuthenticatedUserOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		repos, resp, err := rh.client.Repositories.ListByAuthenticatedUser(ctx, opt)
		if err != nil {
			return err
		}
		for _, repo := range repos {
			if rh.matchRepo(repo) {
				ch <- repo
			}
		}
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	return nil
}

func (rh *RepositoryExecutor) matchRepo(repo *github.Repository) bool {
	if rh.nameRegexp != nil {
		if !rh.nameRegexp.MatchString(repo.GetName()) {
			return false
		}
	}
	if rh.nameSet != nil {
		if _, ok := rh.nameSet[repo.GetName()]; !ok {
			return false
		}
	}
	if rh.topicRegexp != nil {
		// pass if any topic matches
		pass := false
		for _, topic := range repo.Topics {
			if rh.topicRegexp.MatchString(topic) {
				pass = true
				break
			}
		}
		if !pass {
			return false
		}
	}
	if rh.topicSet != nil {
		// pass if any topic matches
		pass := false
		for _, topic := range repo.Topics {
			if _, ok := rh.topicSet[topic]; ok {
				pass = true
				break
			}
		}
		if !pass {
			return false
		}
	}
	return true
}

func (rh *RepositoryExecutor) execCommand(command string, dir string, stdout, stderr io.Writer) error {
	cmd := exec.Command(rh.shellPath, "-c", command)
	cmd.Dir = dir
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func (rh *RepositoryExecutor) cloneRepo(ctx context.Context, dest string, repo *github.Repository) error {
	var auth *http.BasicAuth
	if rh.authUser != nil && rh.authToken != nil {
		auth = &http.BasicAuth{
			Username: *rh.authUser,
			Password: *rh.authToken,
		}
	}
	_, err := git.PlainCloneContext(ctx, dest, false, &git.CloneOptions{
		URL:  repo.GetCloneURL(),
		Auth: auth,
	})
	if err != nil {
		return err
	}
	return nil
}
