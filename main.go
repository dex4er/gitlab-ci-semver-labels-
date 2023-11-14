package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/hashicorp/logutils"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
	"github.com/spf13/viper"
	gitlab "github.com/xanzy/go-gitlab"

	"github.com/dex4er/gitlab-ci-semver-labels/git"
	"github.com/dex4er/gitlab-ci-semver-labels/semver"
)

var version = "dev"

type handleSemverLabelsParams struct {
	BumpInitial           bool
	BumpPatch             bool
	BumpMinor             bool
	BumpMajor             bool
	CommitMessageRegexp   string
	Current               bool
	DotenvFile            string
	DotenvVar             string
	Fail                  bool
	FetchTags             bool
	GitlabTokenEnv        string
	GitlabUrl             string
	InitialLabelRegexp    string
	InitialVersion        string
	MajorLabelRegexp      string
	MinorLabelRegexp      string
	PatchLabelRegexp      string
	Prerelease            bool
	PrereleaseLabelRegexp string
	Project               string
	RemoteName            string
	WorkTree              string
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
	if configFile := viper.ConfigFileUsed(); configFile != "" {
		log.Println("[DEBUG] Config file:", configFile)
	}

	genMarkdown := ""

	params := handleSemverLabelsParams{}

	rootCmd := &cobra.Command{
		Use:     "gitlab-ci-semver-labels",
		Short:   "Bump the semver for a Gitlab CI project based on merge request labels",
		Version: "v" + version,
		RunE: func(cmd *cobra.Command, args []string) error {
			if genMarkdown != "" {
				if err := doc.GenMarkdownTree(cmd, genMarkdown); err != nil {
					fmt.Println("Error:", err)
					os.Exit(2)
				}
				return nil
			}

			return errors.New("missing command")
		},
	}

	rootCmd.PersistentFlags().StringP("dotenv-file", "d", "", "write dotenv format to `FILE`")
	rootCmd.PersistentFlags().StringP("dotenv-var", "D", "VERSION", "variable `NAME` in dotenv file")
	rootCmd.PersistentFlags().BoolP("fetch-tags", "T", true, "fetch tags from git repo")
	rootCmd.PersistentFlags().StringP("gitlab-token-env", "t", "GITLAB_TOKEN", "name for environment `VAR` with Gitlab token")
	rootCmd.PersistentFlags().StringP("gitlab-url", "g", "https://gitlab.com", "`URL` of the Gitlab instance")
	rootCmd.PersistentFlags().StringP("project", "p", "", "`PROJECT` id or name (default $CI_PROJECT_ID)")
	rootCmd.PersistentFlags().StringP("remote-name", "r", "origin", "`NAME` of git remote")
	rootCmd.PersistentFlags().StringP("work-tree", "C", ".", "`DIR` to be used for git operations")

	for _, flag := range []string{
		"dotenv-file",
		"dotenv-var",
		"fetch-tags",
		"gitlab-token-env",
		"gitlab-url",
		"project",
		"remote-name",
		"work-tree",
	} {
		if err := viper.BindPFlag(flag, rootCmd.PersistentFlags().Lookup(flag)); err != nil {
			fmt.Println("Error: incorrect config file:", err)
			os.Exit(1)
		}
	}

	rootCmd.CompletionOptions.DisableDefaultCmd = true

	rootCmd.Flags().StringVar(&genMarkdown, "gen-markdown", "", "Generate Markdown documentation")

	if err := rootCmd.Flags().MarkHidden("gen-markdown"); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	bumpCmd := &cobra.Command{
		Use:   "bump",
		Short: "Bump version",
		RunE: func(cmd *cobra.Command, args []string) error {
			params.CommitMessageRegexp = viper.GetString("commit-message-regexp")
			params.DotenvFile = viper.GetString("dotenv-file")
			params.DotenvVar = viper.GetString("dotenv-var")
			params.Fail = viper.GetBool("fail")
			params.FetchTags = viper.GetBool("fetch-tags")
			params.GitlabTokenEnv = viper.GetString("gitlab-token-env")
			params.GitlabUrl = viper.GetString("gitlab-url")
			params.InitialLabelRegexp = viper.GetString("initial-label-regexp")
			params.InitialVersion = viper.GetString("initial-version")
			params.MajorLabelRegexp = viper.GetString("major-label-regexp")
			params.MinorLabelRegexp = viper.GetString("minor-label-regexp")
			params.PatchLabelRegexp = viper.GetString("patch-label-regexp")
			params.PrereleaseLabelRegexp = viper.GetString("prerelease-label-regexp")
			params.RemoteName = viper.GetString("remote-name")
			params.Project = viper.GetString("project")
			params.WorkTree = viper.GetString("work-tree")

			if err := handleSemverLabels(params); err != nil {
				fmt.Println("Error:", err)
				os.Exit(2)
			}
			return nil
		},
	}

	bumpCmd.Flags().String("commit-message-regexp", `(?s)(?:^|\n)See merge request (?:\w[\w.+/-]*)?!(\d+)`, "`REGEXP` for commit message after merged MR")
	bumpCmd.Flags().BoolP("fail", "f", false, "fail if labels are not matched")
	bumpCmd.Flags().String("initial-label-regexp", "(?i)initial.release|semver(.|::)initial", "`REGEXP` for initial release label")
	bumpCmd.Flags().StringP("initial-version", "V", "0.0.0", "initial `VERSION` for initial release")
	bumpCmd.Flags().String("major-label-regexp", "(?i)(major|breaking).release|semver(.|::)(major|breaking)", "`REGEXP` for major (breaking) release label")
	bumpCmd.Flags().String("minor-label-regexp", "(?i)(minor|feature).release|semver(.|::)(minor|feature)", "`REGEXP` for minor (feature) release label")
	bumpCmd.Flags().String("patch-label-regexp", "(?i)(patch|fix).release|semver(.|::)(patch|fix)", "`REGEXP` for patch (fix) release label")
	bumpCmd.PersistentFlags().BoolP("prerelease", "P", false, "bump version as prerelease")
	bumpCmd.Flags().String("prerelease-label-regexp", "(?i)pre.?release", "`REGEXP` for prerelease label")

	for _, flag := range []string{
		"commit-message-regexp",
		"fail",
		"initial-label-regexp",
		"initial-version",
		"major-label-regexp",
		"minor-label-regexp",
		"patch-label-regexp",
		"prerelease-label-regexp",
	} {
		if err := viper.BindPFlag(flag, bumpCmd.Flags().Lookup(flag)); err != nil {
			fmt.Println("Error: incorrect config file:", err)
			os.Exit(1)
		}
	}

	for _, flag := range []string{
		"prerelease",
	} {
		if err := viper.BindPFlag(flag, bumpCmd.PersistentFlags().Lookup(flag)); err != nil {
			fmt.Println("Error: incorrect config file:", err)
			os.Exit(1)
		}
	}

	bumpInitialCmd := &cobra.Command{
		Use:   "initial",
		Short: "Set to initial version without checking labels",
		RunE: func(cmd *cobra.Command, args []string) error {
			params.BumpInitial = true

			params.DotenvFile = viper.GetString("dotenv-file")
			params.DotenvVar = viper.GetString("dotenv-var")
			params.FetchTags = viper.GetBool("fetch-tags")
			params.GitlabTokenEnv = viper.GetString("gitlab-token-env")
			params.GitlabUrl = viper.GetString("gitlab-url")
			params.InitialVersion = viper.GetString("initial-version")
			params.RemoteName = viper.GetString("remote-name")
			params.Project = viper.GetString("project")
			params.WorkTree = viper.GetString("work-tree")

			if err := handleSemverLabels(params); err != nil {
				fmt.Println("Error:", err)
				os.Exit(2)
			}
			return nil
		},
	}

	bumpInitialCmd.Flags().StringP("initial-version", "V", "0.0.0", "initial `VERSION` for initial release")

	for _, flag := range []string{
		"initial-version",
	} {
		if err := viper.BindPFlag(flag, bumpInitialCmd.Flags().Lookup(flag)); err != nil {
			fmt.Println("Error: incorrect config file:", err)
			os.Exit(1)
		}
	}

	bumpCmd.AddCommand(bumpInitialCmd)

	bumpMajorCmd := &cobra.Command{
		Use:   "major",
		Short: "Bump major version without checking labels",
		RunE: func(cmd *cobra.Command, args []string) error {
			params.BumpMajor = true

			params.DotenvFile = viper.GetString("dotenv-file")
			params.DotenvVar = viper.GetString("dotenv-var")
			params.FetchTags = viper.GetBool("fetch-tags")
			params.GitlabTokenEnv = viper.GetString("gitlab-token-env")
			params.GitlabUrl = viper.GetString("gitlab-url")
			params.RemoteName = viper.GetString("remote-name")
			params.Prerelease = viper.GetBool("prerelease")
			params.Project = viper.GetString("project")
			params.WorkTree = viper.GetString("work-tree")

			if err := handleSemverLabels(params); err != nil {
				fmt.Println("Error:", err)
				os.Exit(2)
			}
			return nil
		},
	}

	bumpCmd.AddCommand(bumpMajorCmd)

	bumpMinorCmd := &cobra.Command{
		Use:   "minor",
		Short: "Bump minor version without checking labels",
		RunE: func(cmd *cobra.Command, args []string) error {
			params.BumpMinor = true

			params.DotenvFile = viper.GetString("dotenv-file")
			params.DotenvVar = viper.GetString("dotenv-var")
			params.FetchTags = viper.GetBool("fetch-tags")
			params.GitlabTokenEnv = viper.GetString("gitlab-token-env")
			params.GitlabUrl = viper.GetString("gitlab-url")
			params.RemoteName = viper.GetString("remote-name")
			params.Prerelease = viper.GetBool("prerelease")
			params.Project = viper.GetString("project")
			params.WorkTree = viper.GetString("work-tree")

			if err := handleSemverLabels(params); err != nil {
				fmt.Println("Error:", err)
				os.Exit(2)
			}
			return nil
		},
	}

	bumpCmd.AddCommand(bumpMinorCmd)

	bumpPatchCmd := &cobra.Command{
		Use:   "patch",
		Short: "Bump patch version without checking labels",
		RunE: func(cmd *cobra.Command, args []string) error {
			params.BumpPatch = true

			params.DotenvFile = viper.GetString("dotenv-file")
			params.DotenvVar = viper.GetString("dotenv-var")
			params.FetchTags = viper.GetBool("fetch-tags")
			params.GitlabTokenEnv = viper.GetString("gitlab-token-env")
			params.GitlabUrl = viper.GetString("gitlab-url")
			params.RemoteName = viper.GetString("remote-name")
			params.Prerelease = viper.GetBool("prerelease")
			params.Project = viper.GetString("project")
			params.WorkTree = viper.GetString("work-tree")

			if err := handleSemverLabels(params); err != nil {
				fmt.Println("Error:", err)
				os.Exit(2)
			}
			return nil
		},
	}

	bumpCmd.AddCommand(bumpPatchCmd)

	rootCmd.AddCommand(bumpCmd)

	currentCmd := &cobra.Command{
		Use:   "current",
		Short: "Show current version",
		RunE: func(cmd *cobra.Command, args []string) error {
			params.Current = true

			params.DotenvFile = viper.GetString("dotenv-file")
			params.DotenvVar = viper.GetString("dotenv-var")
			params.FetchTags = viper.GetBool("fetch-tags")
			params.GitlabTokenEnv = viper.GetString("gitlab-token-env")
			params.GitlabUrl = viper.GetString("gitlab-url")
			params.RemoteName = viper.GetString("remote-name")
			params.WorkTree = viper.GetString("work-tree")

			if err := handleSemverLabels(params); err != nil {
				fmt.Println("Error:", err)
				os.Exit(2)
			}
			return nil
		},
	}

	rootCmd.AddCommand(currentCmd)

	if err := viper.BindEnv("gitlab-url", "CI_SERVER_URL"); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	if err := viper.BindEnv("project", "CI_PROJECT_ID"); err != nil {
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

	log.Println("[DEBUG] Find last tag for remote:", params.RemoteName)
	if params.FetchTags {
		log.Println("[DEBUG] Fetch tags")
	}

	tag, err := git.FindLastTag(git.FindLastTagParams{
		RepositoryPath: params.WorkTree,
		RemoteName:     params.RemoteName,
		GitlabToken:    gitlabToken,
		FetchTags:      params.FetchTags,
	})

	if err != nil {
		return fmt.Errorf("cannot find the last git tag: %w", err)
	}

	log.Printf("[DEBUG] Most recent tag: %v", tag)

	if params.Current {
		ver, err := semver.Current(tag)
		if err != nil {
			return fmt.Errorf("current tag (%s) is not semver: %w", tag, err)
		}
		return printVersion(ver, params.DotenvFile, params.DotenvVar)
	}

	if params.BumpInitial {
		if tag != "" {
			return errors.New("semver is already initialized")
		}

		ver := params.InitialVersion

		if params.Prerelease {
			ver, err = semver.BumpPrerelease(ver)
		}
		if err != nil {
			return fmt.Errorf("cannot bump tag: %w", err)
		}

		return printVersion(ver, params.DotenvFile, params.DotenvVar)
	}

	if params.BumpPatch || params.BumpMinor || params.BumpMajor {
		if tag == "" {
			return errors.New("no tag found")
		}

		ver := tag
		var err error

		if params.BumpPatch {
			ver, err = semver.BumpPatch(ver, params.Prerelease)
		}

		if params.BumpMinor {
			ver, err = semver.BumpMinor(ver, params.Prerelease)
		}

		if params.BumpMajor {
			ver, err = semver.BumpMajor(ver, params.Prerelease)
		}

		if err != nil {
			return fmt.Errorf("cannot bump tag: %w", err)
		}

		return printVersion(ver, params.DotenvFile, params.DotenvVar)
	}

	var labels gitlab.Labels

	mergeRequestLabels := os.Getenv("CI_MERGE_REQUEST_LABELS")

	if mergeRequestLabels != "" {
		labels = strings.Split(mergeRequestLabels, ",")
	} else {
		commitMessage := os.Getenv("CI_COMMIT_MESSAGE")

		re_mr, err := regexp.Compile(params.CommitMessageRegexp)
		if err != nil {
			return err
		}
		matches := re_mr.FindStringSubmatch(commitMessage)

		if len(matches) < 2 {
			log.Println("[WARNING] Merge request not found")
			return nil
		}

		mergeRequest, err := strconv.Atoi(matches[1])
		if err != nil {
			return fmt.Errorf("merge request number is invalid: %w", err)
		}
		log.Println("[DEBUG] Merge request:", mergeRequest)

		log.Println("[DEBUG] GitLab URL:", params.GitlabUrl)
		gl, err := gitlab.NewClient(gitlabToken, gitlab.WithBaseURL(params.GitlabUrl))
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}

		log.Println("[DEBUG] Project:", params.Project)
		opt := &gitlab.GetMergeRequestsOptions{}
		mr, _, err := gl.MergeRequests.GetMergeRequest(params.Project, mergeRequest, opt)

		if err != nil {
			return fmt.Errorf("failed to get information about merge request: %w", err)
		}

		log.Println("[DEBUG] Found merge request:", mr)

		labels = mr.Labels
	}

	log.Println("[DEBUG] Labels:", labels)

	re_initial, err := regexp.Compile(params.InitialLabelRegexp)
	if err != nil {
		return err
	}
	re_major, err := regexp.Compile(params.MajorLabelRegexp)
	if err != nil {
		return err
	}
	re_minor, err := regexp.Compile(params.MinorLabelRegexp)
	if err != nil {
		return err
	}
	re_patch, err := regexp.Compile(params.PatchLabelRegexp)
	if err != nil {
		return err
	}
	re_prerelease, err := regexp.Compile(params.PrereleaseLabelRegexp)
	if err != nil {
		return err
	}

	var ver string
	prerelease := false

	for _, label := range labels {
		if re_prerelease.MatchString(label) {
			log.Println("[DEBUG] Bump: prerelease")
			prerelease = true
		}
	}

	for _, label := range labels {
		if re_initial.MatchString(label) {
			log.Println("[DEBUG] Bump: initial")
			if tag != "" {
				return errors.New("semver is already initialized")
			}
			if ver != "" {
				return errors.New("more than 1 semver label")
			}
			ver = params.InitialVersion
			if prerelease {
				ver, err = semver.BumpPrerelease(ver)
			}
			if err != nil {
				return fmt.Errorf("cannot bump tag: %w", err)
			}
		}
		if re_major.MatchString(label) {
			log.Println("[DEBUG] Bump: major")
			if tag == "" {
				return errors.New("no tag found")
			}
			if ver != "" {
				return errors.New("more than 1 semver label")
			}
			ver, err = semver.BumpMajor(tag, prerelease)
			if err != nil {
				return fmt.Errorf("cannot bump tag: %w", err)
			}
		}
		if re_minor.MatchString(label) {
			log.Println("[DEBUG] Bump: minor")
			if tag == "" {
				return errors.New("no tag found")
			}
			if ver != "" {
				return errors.New("more than 1 semver label")
			}
			ver, err = semver.BumpMinor(tag, prerelease)
			if err != nil {
				return fmt.Errorf("cannot bump tag: %w", err)
			}
		}
		if re_patch.MatchString(label) {
			log.Println("[DEBUG] Bump: patch")
			if tag == "" {
				return errors.New("no tag found")
			}
			if ver != "" {
				return errors.New("more than 1 semver label")
			}
			ver, err = semver.BumpPatch(tag, prerelease)
			if err != nil {
				return fmt.Errorf("cannot bump tag: %w", err)
			}
		}
	}

	if params.Fail && ver == "" {
		return errors.New("no label matched")
	}

	return printVersion(ver, params.DotenvFile, params.DotenvVar)
}
