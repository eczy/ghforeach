# ghforeach

`ghforeach` is a tool to run batch operations on a large set of GitHub repositories. For example, this can be useful for creating or editing matching configuration files across many repos.

## Installation

`make install` will compile the executable and copy it to `/usr/local/bin`. If you want to place the executable elsewhere, run `make build` and then copy it to your desired location.

## Usage

```
Usage: ghforeach [--authuser AUTHUSER] [--authtoken AUTHTOKEN] [--org ORG] [--user USER] [--nameexp NAMEEXP] [--namelist NAMELIST] [--topicexp TOPICEXP] [--topiclist TOPICLIST] [--shell SHELL] [--tmpdir TMPDIR] [--cleanup] [--overwrite] [--nthreads NTHREADS] [--json] [--debug] [COMMAND]

Positional arguments:
  COMMAND                command to run at root of each repo.

Options:
  --authuser AUTHUSER    user for authenticating API requests. [env: GH_AUTH_USER]
  --authtoken AUTHTOKEN
                         token for authenticating API requests. [env: GH_AUTH_TOKEN]
  --org ORG, -o ORG      organization owning repositories to be iterated.
  --user USER, -u USER   user owning repositories to be iterated.
  --nameexp NAMEEXP, -n NAMEEXP
                         regular expression for matching repository names.
  --namelist NAMELIST, -N NAMELIST
                         path to file containing repository names (newline separated).
  --topicexp TOPICEXP, -t TOPICEXP
                         regular expression for matching topics.
  --topiclist TOPICLIST, -T TOPICLIST
                         path to file containing topics (newline separated).
  --shell SHELL, -s SHELL
                         path to shell used to run command. [default: /bin/sh]
  --tmpdir TMPDIR, -d TMPDIR
                         directory into which repositories will be cloned. [default: ./tmp]
  --cleanup, -c          enable to delete TMPDIR after operations are complete.
  --overwrite, -O        enable to delete TMPDIR before operations start.
  --nthreads NTHREADS, -p NTHREADS
                         number of repositories that will be handled in parallel. -1 for unlimited. [default: 1]
  --json, -j             enable to display output as JSON.
  --debug, -D            enable to debug logging.
  --help, -h             display this help and exit
```

If both `user` and `org` are specified, `org` takes precedence. If `command` contains spaces (e.g. `ls -la`), wrap it in double quotes.
