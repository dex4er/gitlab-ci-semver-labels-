package main

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/hashicorp/logutils"
	"github.com/spf13/cobra"
	"github.com/thediveo/enumflag"
	"github.com/xanzy/go-gitlab"

	"github.com/dex4er/gitlab-ci-semver-labels/git"
	"github.com/dex4er/gitlab-ci-semver-labels/semver"
)

var version = "0.0.0"

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
		Version: version,
		RunE:    rootCmdRun,
	}

	rootCmd.Flags().StringP("work-tree", "C", ".", "`DIR` to be used for git operations")
	rootCmd.Flags().StringP("remote-name", "r", "origin", "`NAME` of git remote")
	rootCmd.Flags().StringP("gitlab-token-env", "t", "GITLAB_TOKEN", "name for environment `VAR` with Gitlab token")
	rootCmd.Flags().Bool("fetch-tags", true, "fetch tags from git repo")
	rootCmd.Flags().Bool("current", false, "show current version")
	rootCmd.Flags().VarP(enumflag.New(&bumpmode, "bump", BumpModeIds, enumflag.EnumCaseInsensitive), "bump", "b", "bump version without checking labels: false, current, initial, prerelease, patch, minor, major")
	rootCmd.Flags().String("initial-label", "(?i)(initial.release|semver-initial)", "`REGEXP` for initial release label")
	rootCmd.Flags().String("prerelease-label", "(?i)(pre.?release)", "`REGEXP` for prerelease label")

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

	gitlabToken := os.Getenv(cmd.Flag("gitlab-token-env").Value.String())

	tag, err := git.FindLastTag(git.FindLastTagParams{
		RepositoryPath: cmd.Flag("work-tree").Value.String(),
		RemoteName:     cmd.Flag("remote-name").Value.String(),
		GitlabToken:    gitlabToken,
		FetchTags:      cmd.Flag("fetch-tags").Value.String() == "true",
	})

	if err != nil {
		log.Fatalf("[ERROR] Can't find the last git tag: %v\n", err)
	}

	if tag == "" {
		log.Println("[DEBUG] No tag found")
		return nil
	}

	if cmd.Flag("current").Value.String() == "true" {
		fmt.Println(tag)
		return nil
	}

	if bumpmode != False {
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
		return nil
	}

	mergeRequestLabels := os.Getenv("CI_MERGE_REQUEST_LABELS")

	if mergeRequestLabels == "" {
		commitMessage := os.Getenv("CI_COMMIT_MESSAGE")

		if !strings.HasPrefix(commitMessage, "Merge branch ") {
			log.Println("[DEBUG] Not a merge commit")
			return nil
		}

		re_mr := regexp.MustCompile(`See merge request \!(\d+)`)
		matches := re_mr.FindStringSubmatch(commitMessage)

		if len(matches) <= 1 {
			fmt.Println("[DEBUG] Merge request not found")
			return nil
		}

		mergeRequest := matches[1]
		fmt.Println("[DEBUG] Merge request:", mergeRequest)

		gl, err := gitlab.NewClient(gitlabToken)
		if err != nil {
			log.Fatalf("[ERROR] Failed to create client: %v\n", err)
		}

		opt := &gitlab.GetMergeRequestsOptions{}
		mr, _, err := gl.MergeRequests.GetMergeRequest(os.Getenv("CI_PROJECT_ID"), 1, opt)

		if err != nil {
			log.Fatalf("[ERROR] %v\n", err)
		}

		log.Printf("[DEBUG] Found merge request: %v\n", mr)

		labels := mr.Labels

		log.Printf("[DEBUG] Labels: %v\n", labels)

		re_initial := regexp.MustCompile(cmd.Flag("initial-label").Value.String())
		re_prerelease := regexp.MustCompile(cmd.Flag("prerelease-label").Value.String())

		var ver string

		for _, label := range labels {
			if re_initial.MatchString(label) {
				if ver != "" {
					log.Fatalln("[ERROR] More than 1 semver label")
				}
				ver = "0.0.0"
			}
			if re_prerelease.MatchString(label) {
				if ver != "" {
					log.Fatalln("[ERROR] More than 1 semver label")
				}
				ver, err = semver.BumpPrerelease(tag)
				if err != nil {
					log.Fatalf("[ERROR] Can't bump tag: %v\n", err)
				}
			}
		}

		fmt.Println(ver)
	}

	return nil
}
