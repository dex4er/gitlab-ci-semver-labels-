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
	"github.com/spf13/viper"
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

type handleSemverLabelsParams struct {
	BumpInitial     bool
	BumpPrerelease  bool
	BumpPatch       bool
	BumpMinor       bool
	BumpMajor       bool
	Current         bool
	DotenvFile      string
	DotenvVar       string
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

func main() {
	logLevel := os.Getenv("GITLAB_CI_SEMVER_LABELS_LOG")
	if logLevel == "" {
		logLevel = "ERROR"
	}

	filter := &logutils.LevelFilter{
		Levels:   []logutils.LogLevel{"TRACE", "DEBUG", "WARNING", "ERROR"},
		MinLevel: logutils.LogLevel(logLevel),
		Writer:   os.Stderr,
	}
	log.SetOutput(filter)

	viper.SetConfigName(".gitlab-ci-semver-labels")
	viper.SetConfigType("yml")
	viper.AddConfigPath(".")
	viper.SetEnvPrefix("GITLAB_CI_SEMVER_LABELS")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()
	err := viper.ReadInConfig()
	if err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// ignore
		} else {
			fmt.Println("Error: cannot read config file", err)
			os.Exit(2)
		}
	}
	log.Println("[DEBUG] Config file used:", viper.ConfigFileUsed())

	genMarkdown := ""

	rootCmdParams := handleSemverLabelsParams{}

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

			rootCmdParams.DotenvFile = viper.GetString("dotenv-file")
			rootCmdParams.DotenvVar = viper.GetString("dotenv-var")
			rootCmdParams.FetchTags = viper.GetBool("fetch-tags")
			rootCmdParams.GitlabTokenEnv = viper.GetString("gitlab-token-env")
			rootCmdParams.InitialLabel = viper.GetString("initial-label")
			rootCmdParams.InitialVersion = viper.GetString("initial-version")
			rootCmdParams.MajorLabel = viper.GetString("major-label")
			rootCmdParams.MinorLabel = viper.GetString("minor-label")
			rootCmdParams.PatchLabel = viper.GetString("patch-label")
			rootCmdParams.PrereleaseLabel = viper.GetString("prerelease-label")
			rootCmdParams.RemoteName = viper.GetString("remote-name")
			rootCmdParams.WorkTree = viper.GetString("work-tree")

			if err := handleSemverLabels(rootCmdParams); err != nil {
				fmt.Println("Error:", err)
				os.Exit(2)
			}
			return nil
		},
	}

	rootCmd.Flags().BoolVar(&rootCmdParams.BumpInitial, "bump-initial", false, "set to initial version without checking labels")
	rootCmd.Flags().BoolVar(&rootCmdParams.BumpMajor, "bump-major", false, "bump major version without checking labels")
	rootCmd.Flags().BoolVar(&rootCmdParams.BumpMinor, "bump-minor", false, "bump minor version without checking labels")
	rootCmd.Flags().BoolVar(&rootCmdParams.BumpPatch, "bump-patch", false, "bump patch version without checking labels")
	rootCmd.Flags().BoolVar(&rootCmdParams.BumpPrerelease, "bump-prerelease", false, "bump prerelease version without checking labels")
	rootCmd.Flags().BoolVarP(&rootCmdParams.Current, "current", "c", false, "show current version")
	rootCmd.Flags().StringP("dotenv-file", "d", "", "write dotenv format to `FILE`")
	rootCmd.Flags().StringP("dotenv-var", "D", "version", "variable `NAME` in dotenv file")
	rootCmd.Flags().BoolP("fetch-tags", "f", true, "fetch tags from git repo")
	rootCmd.Flags().StringP("gitlab-token-env", "t", "GITLAB_TOKEN", "name for environment `VAR` with Gitlab token")
	rootCmd.Flags().String("initial-label", "(?i)(initial.release|semver.initial)", "`REGEXP` for initial release label")
	rootCmd.Flags().String("initial-version", "0.0.0", "initial `VERSION` for initial release")
	rootCmd.Flags().String("major-label", "(?i)(major.release|breaking.release|semver.major|semver.breaking)", "`REGEXP` for major (breaking) release label")
	rootCmd.Flags().String("minor-label", "(?i)(minor.release|feature.release|semver.initial|semver.feature)", "`REGEXP` for minor (feature) release label")
	rootCmd.Flags().String("patch-label", "(?i)(patch.release|fix.release|semver.initial|semver.fix)", "`REGEXP` for patch (fix) release label")
	rootCmd.Flags().String("prerelease-label", "(?i)(pre.?release)", "`REGEXP` for prerelease label")
	rootCmd.Flags().StringP("remote-name", "r", "origin", "`NAME` of git remote")
	rootCmd.Flags().StringP("work-tree", "C", ".", "`DIR` to be used for git operations")

	rootCmd.MarkFlagsMutuallyExclusive(
		"bump-initial",
		"bump-major",
		"bump-minor",
		"bump-patch",
		"bump-prerelease",
		"current",
	)

	if err := viper.BindPFlag("dotenv-file", rootCmd.Flags().Lookup("dotenv-file")); err != nil {
		fmt.Println("Error: incorrect config file:", err)
		os.Exit(1)
	}

	if err := viper.BindPFlag("dotenv-env", rootCmd.Flags().Lookup("dotenv-env")); err != nil {
		fmt.Println("Error: incorrect config file:", err)
		os.Exit(1)
	}

	if err := viper.BindPFlag("fetch-tags", rootCmd.Flags().Lookup("fetch-tags")); err != nil {
		fmt.Println("Error: incorrect config file:", err)
		os.Exit(1)
	}

	rootCmd.Flags().StringVar(&genMarkdown, "gen-markdown", "", "Generate Markdown documentation")

	if err := rootCmd.Flags().MarkHidden("gen-markdown"); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func printVersion(ver string, dotenvFile string, dotenvVar string) error {
	if dotenvFile != "" {
		file, err := os.Create(dotenvFile)
		if err != nil {
			return fmt.Errorf("cannot create file: %w", err)
		}
		defer file.Close()
		_, err = file.WriteString(fmt.Sprintf("%s=%s\n", dotenvVar, ver))
		if err != nil {
			return fmt.Errorf("cannot write to file: %w", err)
		}
		log.Println("[DEBUG] Written to file:", dotenvFile)
	}
	_, err := fmt.Println(ver)
	return err
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
		if tag != "" {
			return printVersion(tag, params.DotenvFile, params.DotenvVar)
		}
	}

	if params.BumpInitial {
		if tag != "" {
			return errors.New("semver is already initialized")
		}
		return printVersion(params.InitialVersion, params.DotenvFile, params.DotenvVar)
	}

	if params.BumpPrerelease || params.BumpPatch || params.BumpMinor || params.BumpMajor {
		if tag == "" {
			return errors.New("no tag found")
		}

		if params.BumpPrerelease {
			ver, err := semver.BumpPrerelease(tag)
			if err != nil {
				return fmt.Errorf("cannot bump tag: %w", err)
			}
			return printVersion(ver, params.DotenvFile, params.DotenvVar)
		}

		if params.BumpPatch {
			ver, err := semver.BumpPatch(tag)
			if err != nil {
				return fmt.Errorf("cannot bump tag: %w", err)
			}
			return printVersion(ver, params.DotenvFile, params.DotenvVar)
		}

		if params.BumpMinor {
			ver, err := semver.BumpMinor(tag)
			if err != nil {
				return fmt.Errorf("cannot bump tag: %w", err)
			}
			return printVersion(ver, params.DotenvFile, params.DotenvVar)
		}

		if params.BumpMajor {
			ver, err := semver.BumpMajor(tag)
			if err != nil {
				return fmt.Errorf("cannot bump tag: %w", err)
			}
			return printVersion(ver, params.DotenvFile, params.DotenvVar)
		}
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
				if tag != "" {
					return errors.New("semver is already initialized")
				}
				if ver != "" {
					return errors.New("more than 1 semver label")
				}
				ver = params.InitialVersion
			}
			if re_major.MatchString(label) {
				if tag == "" {
					return errors.New("no tag found")
				}
				if ver != "" {
					return errors.New("more than 1 semver label")
				}
				ver, err = semver.BumpMajor(tag)
				if err != nil {
					return fmt.Errorf("cannot bump tag: %w", err)
				}
			}
			if re_minor.MatchString(label) {
				if tag == "" {
					return errors.New("no tag found")
				}
				if ver != "" {
					return errors.New("more than 1 semver label")
				}
				ver, err = semver.BumpMinor(tag)
				if err != nil {
					return fmt.Errorf("cannot bump tag: %w", err)
				}
			}
			if re_patch.MatchString(label) {
				if tag == "" {
					return errors.New("no tag found")
				}
				if ver != "" {
					return errors.New("more than 1 semver label")
				}
				ver, err = semver.BumpPatch(tag)
				if err != nil {
					return fmt.Errorf("cannot bump tag: %w", err)
				}
			}
			if re_prerelease.MatchString(label) {
				if tag == "" {
					return errors.New("no tag found")
				}
				if ver != "" {
					return errors.New("more than 1 semver label")
				}
				ver, err = semver.BumpPrerelease(tag)
				if err != nil {
					return fmt.Errorf("cannot bump tag: %w", err)
				}
			}
		}

		return printVersion(ver, params.DotenvFile, params.DotenvVar)
	}

	return nil
}
