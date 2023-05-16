package main

import (
	"fmt"
	"log"
	"os"

	"github.com/hashicorp/logutils"
	"github.com/spf13/cobra"
	"github.com/thediveo/enumflag"

	"github.com/dex4er/gitlab-ci-semver-labels/git"
	"github.com/dex4er/gitlab-ci-semver-labels/semver"
)

var VERSION = "0.0.0"

type BumpMode enumflag.Flag

const (
	False BumpMode = iota
	Initial
	Prerelease
	Patch
	Minor
	Major
)

var BumpModeIds = map[BumpMode][]string{
	False:      {"false"},
	Initial:    {"initial"},
	Prerelease: {"prerelease"},
	Patch:      {"patch"},
	Minor:      {"minor"},
	Major:      {"major"},
}

var bumpmode BumpMode

func main() {
	rootCmd := &cobra.Command{
		Use:     "gitlab-ci-semver-labels",
		Short:   "Bump the semver for a Gitlab CI project",
		Version: VERSION,
		RunE:    rootCmdRun,
	}

	rootCmd.Flags().StringP("work-tree", "C", ".", "`DIR` to be used for git operations")
	rootCmd.Flags().StringP("remote-name", "r", "origin", "`NAME` of git remote")
	rootCmd.Flags().StringP("gitlab-token-env", "t", "GITLAB_TOKEN", "name for environment `VAR` with Gitlab token")
	rootCmd.Flags().Bool("fetch-tags", true, "fetch tags from git repo")

	rootCmd.Flags().VarP(enumflag.New(&bumpmode, "bump", BumpModeIds, enumflag.EnumCaseInsensitive), "bump", "b", "bump version without checking labels: initial, prerelease, patch, minor, major")

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func rootCmdRun(cmd *cobra.Command, args []string) error {
	logLevel := os.Getenv("LOGLEVEL")
	if logLevel == "" {
		logLevel = "ERROR"
	}

	filter := &logutils.LevelFilter{
		Levels:   []logutils.LogLevel{"DEBUG", "WARNING", "ERROR"},
		MinLevel: logutils.LogLevel(logLevel),
		Writer:   os.Stderr,
	}
	log.SetOutput(filter)

	tag, err := git.FindLastTag(git.FindLastTagParams{
		RepositoryPath: cmd.Flag("work-tree").Value.String(),
		RemoteName:     cmd.Flag("remote-name").Value.String(),
		GitlabToken:    os.Getenv(cmd.Flag("gitlab-token-env").Value.String()),
		FetchTags:      cmd.Flag("fetch-tags").Value.String() == "true",
	})

	if err != nil {
		log.Fatalf("[ERROR] Can't find the last git tag: %v\n", err)
	}

	if tag == "" {
		log.Println("[DEBUG] No tag found")
		return nil
	}

	if bumpmode != 0 {
		log.Printf("[DEBUG] Bump mode %v\n", bumpmode)

		var ver string
		if bumpmode == Initial {
			ver = "0.0.0"
		} else if bumpmode == Prerelease {
			ver, err = semver.BumpPrerelease(tag)
			if err != nil {
				log.Fatalf("[ERROR] Can't bump tag: %v\n", err)
			}
		} else if bumpmode == Patch {
			ver, err = semver.BumpPatch(tag)
			if err != nil {
				log.Fatalf("[ERROR] Can't bump tag: %v\n", err)
			}
		} else if bumpmode == Minor {
			ver, err = semver.BumpMinor(tag)
			if err != nil {
				log.Fatalf("[ERROR] Can't bump tag: %v\n", err)
			}
		} else if bumpmode == Major {
			ver, err = semver.BumpMajor(tag)
			if err != nil {
				log.Fatalf("[ERROR] Can't bump tag: %v\n", err)
			}
		}
		fmt.Println(ver)
	}

	return nil
}
