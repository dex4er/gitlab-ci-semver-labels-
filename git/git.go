package git

import (
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

func FindLastTag(repositoryPath string, gitlabToken string) (string, error) {
	// Open the repository
	repo, err := git.PlainOpen(repositoryPath)
	if err != nil {
		return "", err
	}

	// Fetch all tags
	err = fetchTags(repo, gitlabToken)
	if err != nil {
		return "", err
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
func fetchTags(repo *git.Repository, accessToken string) (error) {
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
	tagRefs, err := repo.Tags()
	if err != nil {
		return "", err
	}

	var mostRecentTag *plumbing.Reference
	var mostRecentCommitTime time.Time 
	err = tagRefs.ForEach(func(ref *plumbing.Reference) error {
		if ref.Type() != plumbing.SymbolicReference {
			tagCommitObj, err := repo.CommitObject(ref.Hash())
			if err != nil {
				return err
			}
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
