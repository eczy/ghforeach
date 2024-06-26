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
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/alexflint/go-arg"
	"github.com/google/go-github/v60/github"
	"go.uber.org/zap"
)

type Args struct {
	Command string `arg:"positional" help:"command to run at root of each repo."`

	// authentication
	AuthUser  *string `arg:"env:GH_AUTH_USER" help:"user for authenticating API requests."`
	AuthToken *string `arg:"env:GH_AUTH_TOKEN" help:"token for authenticating API requests."`

	// repo owner options
	Org  *string `arg:"-o" help:"organization owning repositories to be iterated."`
	User *string `arg:"-u" help:"user owning repositories to be iterated."`

	// filtering parameters
	NameExp   *string `arg:"-n" help:"regular expression for matching repository names."`
	NameList  *string `arg:"-N" help:"path to file containing repository names (newline separated)."`
	TopicExp  *string `arg:"-t" help:"regular expression for matching topics."`
	TopicList *string `arg:"-T" help:"path to file containing topics (newline separated)."`

	// execution parameters
	Shell     string `arg:"-s" default:"/bin/sh" help:"path to shell used to run command."`
	TmpDir    string `arg:"-d" default:"./tmp" help:"directory into which repositories will be cloned."`
	Cleanup   bool   `arg:"-c" help:"enable to delete TMPDIR after operations are complete."`
	Overwrite bool   `arg:"-O" help:"enable to delete TMPDIR before operations start."`
	NThreads  int    `arg:"-p" default:"1" help:"number of repositories that will be handled in parallel. -1 for unlimited."`
	Json      bool   `arg:"-j" help:"enable to display output as JSON."`
	Debug     bool   `arg:"-D" help:"enable to debug logging."`
}

func Run() error {
	args := &Args{}
	arg.MustParse(args)
	return RunWithArgs(args)
}

func RunWithArgs(args *Args) error {
	var logger *zap.Logger
	if args.Debug {
		l, err := zap.NewDevelopment()
		if err != nil {
			return err
		}
		logger = l
	} else {
		l, err := zap.NewProduction()
		if err != nil {
			return err
		}
		logger = l
	}
	defer logger.Sync()

	client := github.NewClient(nil)
	if args.AuthToken != nil {
		logger.Debug("reading GH_AUTH_TOKEN token from env")
		client = client.WithAuthToken(*args.AuthToken)
	}

	ctx := context.Background()

	opts := []RepositoryExecutorOption{
		WithClient(client),
		WithLogger(logger),
		WithCleanup(args.Cleanup),
		WithOverwrite(args.Overwrite),
		WithConcurrency(args.NThreads),
		WithTmpDir(args.TmpDir),
		WithShellPath(args.Shell),
	}

	if args.AuthUser != nil && args.AuthToken != nil {
		opts = append(opts, WithUserAuth(*args.AuthToken, *args.AuthToken))
	}
	if args.Org != nil {
		opts = append(opts, WithOrg(*args.Org))
	}
	if args.User != nil {
		opts = append(opts, WithUser(*args.User))
	}
	if args.NameExp != nil {
		opts = append(opts, WithNameRegexp(*args.NameExp))
	}
	if args.TopicExp != nil {
		opts = append(opts, WithTopicRegexp(*args.TopicExp))
	}
	if args.NameList != nil {
		bytes, err := os.ReadFile(*args.NameList)
		if err != nil {
			return err
		}
		nameList := strings.Split(string(bytes), "\n")
		opts = append(opts, WithNameList(nameList))
	}
	if args.TopicList != nil {
		bytes, err := os.ReadFile(*args.TopicList)
		if err != nil {
			return err
		}
		topicList := strings.Split(string(bytes), "\n")
		opts = append(opts, WithTopicList(topicList))
	}
	if args.Json {
		opts = append(opts, WithOutputFormat(JsonOutputFormat))
	}

	handler, err := NewRepositoryExecutor(opts...)
	if err != nil {
		return err
	}
	if len(args.Command) == 0 {
		return fmt.Errorf("no command provided")
	}
	return handler.Go(ctx, args.Command)
}
