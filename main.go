package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/hashicorp/logutils"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
	"github.com/thediveo/enumflag"
	"github.com/xanzy/go-gitlab"

	"github.com/dex4er/gitlab-ci-semver-labels/git"
	"github.com/dex4er/gitlab-ci-semver-labels/semver"
)

var version = "dev"

type SemverBump enumflag.Flag

const (
	BumpFalse SemverBump = iota
	BumpInitial
	BumpPrerelease
	BumpPatch
	BumpMinor
	BumpMajor
)

var SemverBumpIds = map[SemverBump][]string{
	BumpFalse:      {"false"},
	BumpInitial:    {"initial"},
	BumpPrerelease: {"prerelease"},
	BumpPatch:      {"patch"},
	BumpMinor:      {"minor"},
	BumpMajor:      {"major"},
}

func main() {
	logLevel := os.Getenv("LOGLEVEL")
	if logLevel == "" {
		logLevel = "ERROR"
	}

	filter := &logutils.LevelFilter{
		Levels:   []logutils.LogLevel{"TRACE", "DEBUG", "WARNING", "ERROR"},
		MinLevel: logutils.LogLevel(logLevel),
		Writer:   os.Stderr,
	}
	log.SetOutput(filter)

	params := handleSemverLabelsParams{}

	genMarkdown := ""

	rootCmd := &cobra.Command{
		Use:     "gitlab-ci-semver-labels",
		Short:   "Bump the semver for a Gitlab CI project",
		Version: "v" + version,
		RunE: func(cmd *cobra.Command, args []string) error {
			if genMarkdown != "" {
				if err := doc.GenMarkdownTree(cmd, genMarkdown); err != nil {
					fmt.Println("Error:", err)
					os.Exit(2)
				}
				return nil
			}
			if err := handleSemverLabels(params); err != nil {
				fmt.Println("Error:", err)
				os.Exit(2)
			}
			return nil
		},
	}

	rootCmd.Flags().VarP(enumflag.New(&params.BumpMode, "bump", SemverBumpIds, enumflag.EnumCaseInsensitive), "bump", "b", "`BUMP` version without checking labels: false, current, initial, prerelease, patch, minor, major")
	rootCmd.Flags().BoolVarP(&params.Current, "current", "c", false, "show current version")
	rootCmd.Flags().StringVarP(&params.Dotenv, "dotenv", "d", "", "write dotenv format to `FILE`")
	rootCmd.Flags().BoolVarP(&params.FetchTags, "fetch-tags", "f", true, "fetch tags from git repo")
	rootCmd.Flags().StringVarP(&params.GitlabTokenEnv, "gitlab-token-env", "t", "GITLAB_TOKEN", "name for environment `VAR` with Gitlab token")
	rootCmd.Flags().StringVar(&params.InitialLabel, "initial-label", "(?i)(initial.release|semver.initial)", "`REGEXP` for initial release label")
	rootCmd.Flags().StringVar(&params.InitialVersion, "initial-version", "0.0.0", "initial `VERSION` for initial release")
	rootCmd.Flags().StringVar(&params.MajorLabel, "major-label", "(?i)(major.release|breaking.release|semver.major|semver.breaking)", "`REGEXP` for major (breaking) release label")
	rootCmd.Flags().StringVar(&params.MinorLabel, "minor-label", "(?i)(minor.release|feature.release|semver.initial|semver.feature)", "`REGEXP` for minor (feature) release label")
	rootCmd.Flags().StringVar(&params.PatchLabel, "patch-label", "(?i)(patch.release|fix.release|semver.initial|semver.fix)", "`REGEXP` for patch (fix) release label")
	rootCmd.Flags().StringVar(&params.PrereleaseLabel, "prerelease-label", "(?i)(pre.?release)", "`REGEXP` for prerelease label")
	rootCmd.Flags().StringVarP(&params.RemoteName, "remote-name", "r", "origin", "`NAME` of git remote")
	rootCmd.Flags().StringVarP(&params.WorkTree, "work-tree", "C", ".", "`DIR` to be used for git operations")

	rootCmd.Flags().StringVar(&genMarkdown, "gen-markdown", "", "Generate Markdown documentation")

	if err := rootCmd.Flags().MarkHidden("gen-markdown"); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func printVersion(ver string, dotenv string) error {
	if dotenv != "" {
		file, err := os.Create(dotenv)
		if err != nil {
			return fmt.Errorf("cannot create file: %w", err)
		}
		defer file.Close()
		_, err = file.WriteString(fmt.Sprintf("version=%s\n", ver))
		if err != nil {
			return fmt.Errorf("cannot write to file: %w", err)
		}
		log.Println("[DEBUG] Written to file:", dotenv)
	}
	_, err := fmt.Println(ver)
	return err
}

type handleSemverLabelsParams struct {
	BumpMode        SemverBump
	Current         bool
	Dotenv          string
	FetchTags       bool
	GitlabTokenEnv  string
	InitialLabel    string
	InitialVersion  string
	MajorLabel      string
	MinorLabel      string
	PatchLabel      string
	PrereleaseLabel string
	RemoteName      string
	WorkTree        string
}

func handleSemverLabels(params handleSemverLabelsParams) error {
	gitlabToken := os.Getenv(params.GitlabTokenEnv)

	tag, err := git.FindLastTag(git.FindLastTagParams{
		RepositoryPath: params.WorkTree,
		RemoteName:     params.RemoteName,
		GitlabToken:    gitlabToken,
		FetchTags:      params.FetchTags,
	})

	if err != nil {
		return fmt.Errorf("cannot find the last git tag: %w", err)
	}

	if params.Current {
		_, err := fmt.Println(tag)
		return err
	}

	if params.BumpMode != BumpFalse {
		log.Println("[DEBUG] Bump mode:", SemverBumpIds[params.BumpMode][0])

		if params.BumpMode == BumpInitial {
			if tag != "" {
				return errors.New("semver is already initialized")
			}
			fmt.Println(params.InitialVersion)
			return nil
		}

		if tag == "" {
			return errors.New("no tag found")
		}

		var ver string

		if params.BumpMode == BumpPrerelease {
			ver, err = semver.BumpPrerelease(tag)
		} else if params.BumpMode == BumpPatch {
			ver, err = semver.BumpPatch(tag)
		} else if params.BumpMode == BumpMinor {
			ver, err = semver.BumpMinor(tag)
		} else if params.BumpMode == BumpMajor {
			ver, err = semver.BumpMajor(tag)
		}
		if err != nil {
			return fmt.Errorf("cannot bump tag: %w", err)
		}

		return printVersion(ver, params.Dotenv)
	}

	mergeRequestLabels := os.Getenv("CI_MERGE_REQUEST_LABELS")

	if mergeRequestLabels == "" {
		commitMessage := os.Getenv("CI_COMMIT_MESSAGE")

		if !strings.HasPrefix(commitMessage, "Merge branch ") {
			log.Println("[WARNING] Not a merge commit")
			return nil
		}

		re_mr := regexp.MustCompile(`See merge request \!(\d+)`)
		matches := re_mr.FindStringSubmatch(commitMessage)

		if len(matches) <= 1 {
			log.Println("[WARNING] Merge request not found")
			return nil
		}

		mergeRequest := matches[1]
		log.Println("[DEBUG] Merge request:", mergeRequest)

		gl, err := gitlab.NewClient(gitlabToken)
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}

		opt := &gitlab.GetMergeRequestsOptions{}
		mr, _, err := gl.MergeRequests.GetMergeRequest(os.Getenv("CI_PROJECT_ID"), 1, opt)

		if err != nil {
			return fmt.Errorf("failed to get information about merge request: %w", err)
		}

		log.Println("[DEBUG] Found merge request:", mr)

		labels := mr.Labels

		log.Println("[DEBUG] Labels:", labels)

		re_initial := regexp.MustCompile(params.InitialLabel)
		re_major := regexp.MustCompile(params.MajorLabel)
		re_minor := regexp.MustCompile(params.MinorLabel)
		re_patch := regexp.MustCompile(params.PatchLabel)
		re_prerelease := regexp.MustCompile(params.PrereleaseLabel)

		var ver string

		for _, label := range labels {
			if re_initial.MatchString(label) {
				if ver != "" {
					return errors.New("more than 1 semver label")
				}
				ver = params.InitialVersion
			}
			if re_major.MatchString(label) {
				if ver != "" {
					return errors.New("more than 1 semver label")
				}
				ver, err = semver.BumpMajor(tag)
				if err != nil {
					return fmt.Errorf("cannot bump tag: %w", err)
				}
			}
			if re_minor.MatchString(label) {
				if ver != "" {
					return errors.New("more than 1 semver label")
				}
				ver, err = semver.BumpMinor(tag)
				if err != nil {
					return fmt.Errorf("cannot bump tag: %w", err)
				}
			}
			if re_patch.MatchString(label) {
				if ver != "" {
					return errors.New("more than 1 semver label")
				}
				ver, err = semver.BumpPatch(tag)
				if err != nil {
					return fmt.Errorf("cannot bump tag: %w", err)
				}
			}
			if re_prerelease.MatchString(label) {
				if ver != "" {
					return errors.New("more than 1 semver label")
				}
				ver, err = semver.BumpPrerelease(tag)
				if err != nil {
					return fmt.Errorf("cannot bump tag: %w", err)
				}
			}
		}

		return printVersion(ver, params.Dotenv)
	}

	return nil
}
