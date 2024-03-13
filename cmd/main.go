package main

import (
	"context"
	"log"
	"os"
	"strings"

	"github.com/alexflint/go-arg"
	"github.com/eczy/ghforeach/internal/ghforeach"
	"github.com/google/go-github/v60/github"
	"go.uber.org/zap"
)

type args struct {
	Command string `arg:"positional" help:"command to run at root of each repo."`

	// authentication
	AuthUser  *string `arg:"env:GH_AUTH_USER" help:"user for authenticating API requests."`
	AuthToken *string `arg:"env:GH_AUTH_TOKEN" help:"token for authenticating API requests."`

	// repo owner options
	Org  *string `arg:"-o" help:"organization owning repositories to be iterated."`
	User *string `arg:"-u" help:"user owning repositories to be iterated."`

	// filtering parameters
	NameExp   *string `arg:"-n" help:"regular expression for matching repository names."`
	NameList  *string `arg:"-N" help:"path to file containing repository names (\n separated)."`
	TopicExp  *string `arg:"-t" help:"regular expression for matching topics."`
	TopicList *string `arg:"-T" help:"path to file containing topics (\n separated)."`

	// execution parameters
	TmpDir    string `arg:"-d" default:"./tmp" help:"directory into which repositories will be cloned."`
	Cleanup   bool   `arg:"-c" help:"enable to delete TMPDIR after operations are complete."`
	Overwrite bool   `arg:"-O" help:"enable to delete TMPDIR before operations start."`
	NThreads  int    `arg:"-p" default:"1" help:"number of repositories that will be handled in parallel. -1 for unlimited."`
	Json      bool   `arg:"-j" help:"enable to display output as JSON."`
	Debug     bool   `arg:"-D" help:"enable to debug logging."`
}

func main() {
	if err := mainErr(); err != nil {
		log.Fatal(err)
	}
}

func mainErr() error {
	args := &args{}
	arg.MustParse(args)

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

	opts := []ghforeach.RepositoryExecutorOption{
		ghforeach.WithClient(client),
		ghforeach.WithLogger(logger),
		ghforeach.WithCleanup(args.Cleanup),
		ghforeach.WithOverwrite(args.Overwrite),
		ghforeach.WithConcurrency(args.NThreads),
		ghforeach.WithTmpDir(args.TmpDir),
	}

	if args.AuthUser != nil && args.AuthToken != nil {
		opts = append(opts, ghforeach.WithUserAuth(*args.AuthToken, *args.AuthToken))
	}
	if args.Org != nil {
		opts = append(opts, ghforeach.WithOrg(*args.Org))
	}
	if args.User != nil {
		opts = append(opts, ghforeach.WithUser(*args.User))
	}
	if args.NameExp != nil {
		opts = append(opts, ghforeach.WithNameExp(*args.NameExp))
	}
	if args.TopicExp != nil {
		opts = append(opts, ghforeach.WithNameExp(*args.TopicExp))
	}
	if args.NameList != nil {
		bytes, err := os.ReadFile(*args.NameList)
		if err != nil {
			return err
		}
		nameList := strings.Split(string(bytes), "\n")
		opts = append(opts, ghforeach.WithNameList(nameList))
	}
	if args.TopicList != nil {
		bytes, err := os.ReadFile(*args.TopicList)
		if err != nil {
			return err
		}
		topicList := strings.Split(string(bytes), "\n")
		opts = append(opts, ghforeach.WithTopicList(topicList))
	}
	if args.Json {
		opts = append(opts, ghforeach.WithOutputFormat(ghforeach.JsonOutputFormat))
	}

	handler, err := ghforeach.NewRepositoryExecutor(opts...)
	if err != nil {
		return err
	}
	logger.Info("running repository handler")
	return handler.Go(args.Command, ctx)
}
