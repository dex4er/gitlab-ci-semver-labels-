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

	"github.com/dex4er/gitlab-ci-semver-labels/semver"
)

type FindLastTagParams struct {
	RepositoryPath string
	RemoteName     string
	GitlabToken    string
	FetchTags      bool
}

func FindLastTag(params FindLastTagParams) (string, error) {
	log.Printf(
		"[TRACE] FindLastTag(RepositoryPath=%v, RemoteName=%v, GitlabToken=%v, FetchTags=%v)",
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

// Find the most recent tag that points to the given commit or annotated tag object
func findMostRecentTagForCommit(repo *git.Repository, commitObj *object.Commit) (string, error) {
	log.Printf("[TRACE] findMostRecentTagForCommit(repo=%v, commitObj=%v)", repo, commitObj)
	tagRefs, err := repo.Tags()
	if err != nil {
		return "", err
	}

	mostRecentTag := ""
	var mostRecentCommitTime time.Time

	err = tagRefs.ForEach(func(ref *plumbing.Reference) error {
		log.Printf("[TRACE] findMostRecentTagForCommit tagRefs.ForEach(ref=%v)", ref)

		var tagTime time.Time

		if ref.Type() != plumbing.SymbolicReference {
			refHash := ref.Hash()
			tagObj, err := repo.TagObject(refHash)

			if err == nil {
				log.Printf("[TRACE] tagObj=%v", tagObj)
				tagTime = tagObj.Tagger.When
			} else {
				commitObj, err := repo.CommitObject(refHash)
				if err == nil {
					log.Printf("[TRACE] commitObj=%v", commitObj)
					tagTime = commitObj.Author.When
				} else {
					log.Printf("[DEBUG] no commit nor annotated tag for a given hash: %s: %v", refHash, err)
					return nil
				}
			}

			tag := ref.Name().Short()
			log.Printf("[DEBUG] Found tag: %s", tag)

			if semver.IsValid(tag) {
				if mostRecentTag == "" {
					mostRecentTag = tag
					mostRecentCommitTime = tagTime
					log.Printf("[TRACE] mostRecentTag=%v", mostRecentTag)
				} else {
					commitTime := tagTime

					if commitTime.After(mostRecentCommitTime) {
						mostRecentTag = tag
						mostRecentCommitTime = commitTime
					}
					log.Printf("[TRACE] mostRecentTag=%v", mostRecentTag)
				}
			} else {
				log.Printf("[WARNING] %v is not a valid semver", tag)
			}
		}

		return nil
	})

	if err != nil {
		return "", err
	}

	return mostRecentTag, nil
}
