package git

import (
	"log"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/usvc/go-semver"
)

type FindLastTagParams struct {
	RepositoryPath string
	RemoteName     string
	GitlabToken    string
	FetchTags      bool
}

func FindLastTag(params FindLastTagParams) (string, error) {
	log.Printf(
		"[DEBUG] FindLastTag(RepositoryPath=%v, RemoteName=%v, GitlabToken=%v, FetchTags=%v)\n",
		params.RepositoryPath,
		params.RemoteName,
		params.GitlabToken,
		params.FetchTags,
	)

	// Open the repository
	repo, err := git.PlainOpen(params.RepositoryPath)
	if err != nil {
		return "", err
	}

	// Fetch all tags
	if params.FetchTags {
		err = fetchTags(repo, params.RemoteName, params.GitlabToken)
		if err != nil {
			return "", err
		}
	}

	// Get the HEAD reference
	ref, err := repo.Head()
	if err != nil {
		return "", err
	}

	// Retrieve the commit object for HEAD
	commitObj, err := repo.CommitObject(ref.Hash())
	if err != nil {
		return "", err
	}

	// Find the most recent tag that points to the HEAD commit
	tag, err := findMostRecentTagForCommit(repo, commitObj)
	if err != nil {
		return "", err
	}

	return tag, nil
}

// Add HTTP Basic Authorization to Git client
func getAuth(accessToken string) transport.AuthMethod {
	if accessToken != "" {
		return &http.BasicAuth{
			Username: "oauth2",
			Password: accessToken,
		}
	}
	return nil
}

// Fetch all tags
func fetchTags(repo *git.Repository, remoteName string, accessToken string) error {
	fetchOptions := &git.FetchOptions{
		RemoteName: "origin",
		RefSpecs:   []config.RefSpec{"+refs/tags/*:refs/tags/*"},
		Auth:       getAuth(accessToken),
	}
	err := repo.Fetch(fetchOptions)
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return err
	}
	return nil
}

// Find the most recent tag that points to the given commit object
func findMostRecentTagForCommit(repo *git.Repository, commitObj *object.Commit) (string, error) {
	log.Printf("[DEBUG] findMostRecentTagForCommit(repo=%v, commitObj=%v)\n", repo, commitObj)
	tagRefs, err := repo.Tags()
	if err != nil {
		return "", err
	}

	var mostRecentTag *plumbing.Reference
	var mostRecentCommitTime time.Time
	err = tagRefs.ForEach(func(ref *plumbing.Reference) error {
		log.Printf("[DEBUG] findMostRecentTagForCommit tagRefs.ForEach(ref=%v)\n", ref)
		if ref.Type() != plumbing.SymbolicReference {
			tagCommitObj, err := repo.CommitObject(ref.Hash())
			if err != nil {
				return err
			}
			tag := ref.Name().Short()
			log.Printf("[DEBUG] findMostRecentTagForCommit tag=%v\n", tag)
			if semver.IsValid(tag) {
				if mostRecentTag == nil {
					mostRecentTag = ref
					mostRecentCommitTime = tagCommitObj.Committer.When
				} else {
					commitTime := tagCommitObj.Committer.When

					if commitTime.After(mostRecentCommitTime) {
						mostRecentTag = ref
						mostRecentCommitTime = commitTime
					}
				}
			} else {
				log.Printf("[WARNING] %v is not a valid semver\n", tag)
			}
		}
		return nil
	})

	if err != nil {
		return "", err
	}

	if mostRecentTag == nil {
		return "", nil
	}

	tagName := mostRecentTag.Name().Short()

	return tagName, nil
}
