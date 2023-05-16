package main

import (
	"fmt"
	"log"
	"os"

	"github.com/hashicorp/logutils"
	"github.com/spf13/cobra"

	"github.com/dex4er/gitlab-ci-semver-labels/git"
)

var VERSION = "0.0.0"

func main() {
	rootCmd := &cobra.Command{
		Use:     "gitlab-ci-semver-labels",
		Short:   "Bump the semver for a Gitlab CI project",
		Version: VERSION,
		RunE:    rootCmd,
	}

	rootCmd.Flags().StringP("work-tree", "C", ".", "`DIR` to be used for git operations")
	rootCmd.Flags().StringP("gitlab-token-env", "t", "GITLAB_TOKEN", "name of the variable with Gitlab token")

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func rootCmd(cmd *cobra.Command, args []string) error {
	repositoryPath := cmd.Flag("work-tree").Value.String()
	gitlabTokenEnv := cmd.Flag("gitlab-token-env").Value.String()

	gitlabToken := os.Getenv(gitlabTokenEnv)

	filter := &logutils.LevelFilter{
		Levels: []logutils.LogLevel{"DEBUG", "ERROR"},
		MinLevel: logutils.LogLevel("ERROR"),
		Writer: os.Stderr,
	}
	log.SetOutput(filter)

	tag, err := git.FindLastTag(repositoryPath, gitlabToken)

	if err != nil {
		log.Fatalf("[ERROR] %v\n", err)
	}

	fmt.Println(tag)

	return nil
}
