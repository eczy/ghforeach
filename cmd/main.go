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
	Command string `arg:"positional"`

	// authentication
	AuthUser  *string `arg:"env:GH_AUTH_USER"`
	AuthToken *string `arg:"env:GH_AUTH_TOKEN"`

	// repo owner options
	Org  *string `arg:"-o"`
	User *string `arg:"-u"`

	// filtering parameters
	NameExp   *string `arg:"-n"`
	NameList  *string `arg:"-N"`
	TopicExp  *string `arg:"-t"`
	TopicList *string `arg:"-T"`

	// execution parameters
	TmpDir    string `arg:"-d" default:"./tmp"`
	Cleanup   bool   `arg:"-c"`
	Overwrite bool   `arg:"-O"`
	NThreads  int    `arg:"-p" default:"1"`
	Json      bool   `arg:"-j"`
	Debug     bool   `arg:"-D"`
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
		opts = append(opts, ghforeach.WithNameList(topicList))
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
