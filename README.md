# gitlab-ci-semver-labels

Bump the semver for a Gitlab CI project based on merge request labels.

If no `--current` option was used nor any of `--bump` options then labels of the
Merge Requests are taken either from `$CI_MERGE_REQUEST_LABELS` environment
variable or from details of the Merge Requests pointed by the commit message
from `$CI_COMMIT_MESSAGE` environment variable.

Commit message should contain the string `See merge request PROJECT!NUMBER`.

To fetch the details of the Merge Request the tool needs the Gitlab API token
with the `read_api` scope. The token is taken from the `$GITLAB_TOKEN`
environment variable by default.

## Usage

```sh
gitlab-ci-semver-labels [flags]
```

### Options

```console
      --bump-initial                     set to initial version without checking labels
      --bump-major                       bump major version without checking labels
      --bump-minor                       bump minor version without checking labels
      --bump-patch                       bump patch version without checking labels
      --bump-prerelease                  bump prerelease version without checking labels
      --commit-message-regexp REGEXP     REGEXP for commit message after merged MR (default "(?s)(?:^|\\n)See merge request (?:\\w[\\w.+/-]*)?!(\\d+)")
  -c, --current                          show current version
  -d, --dotenv-file FILE                 write dotenv format to FILE
  -D, --dotenv-var NAME                  variable NAME in dotenv file (default "version")
  -f, --fail                             fail if merge request are not matched
  -T, --fetch-tags                       fetch tags from git repo (default true)
  -t, --gitlab-token-env VAR             name for environment VAR with Gitlab token (default "GITLAB_TOKEN")
  -g, --gitlab-url URL                   URL of the Gitlab instance (default "https://gitlab.com")
  -h, --help                             help for gitlab-ci-semver-labels
      --initial-label-regexp REGEXP      REGEXP for initial release label (default "(?i)(initial.release|semver.initial)")
      --initial-version VERSION          initial VERSION for initial release (default "0.0.0")
      --major-label-regexp REGEXP        REGEXP for major (breaking) release label (default "(?i)(major.release|breaking.release|semver.major|semver.breaking)")
      --minor-label-regexp REGEXP        REGEXP for minor (feature) release label (default "(?i)(minor.release|feature.release|semver.initial|semver.feature)")
      --patch-label-regexp REGEXP        REGEXP for patch (fix) release label (default "(?i)(patch.release|fix.release|semver.initial|semver.fix)")
      --prerelease-label-regexp REGEXP   REGEXP for prerelease label (default "(?i)(pre.?release)")
  -p, --project PROJECT                  PROJECT id or name (default $CI_PROJECT_ID)
  -r, --remote-name NAME                 NAME of git remote (default "origin")
  -v, --version                          version for gitlab-ci-semver-labels
  -C, --work-tree DIR                    DIR to be used for git operations (default ".")
```

### Configuration

Some options can be read from the configuration file
`.gitlab-ci-semver-labels.yml`:

```yaml
commit-message-regexp: (?s)(?:^|\n)See merge request (?:\w[\w.+/-]*)?!(\d+)
dotenv-file: ""
dotenv-var: version
fail: false
fetch-tags: true
gitlab-token-env: GITLAB_TOKEN
gitlab-url: https://gitlab.com
initial-label-regexp: (?i)(initial.release|semver.initial)
initial-version: 0.0.0
major-label-regexp: (?i)(major.release|breaking.release|semver.major|semver.breaking)
minor-label-regexp: (?i)(minor.release|feature.release|semver.initial|semver.feature)
patch-label-regexp: (?i)(patch.release|fix.release|semver.initial|semver.fix)
prerelease-label-regexp: (?i)(pre.?release)
project: dex4er/gitlab-ci-semver-labels
remote-name: origin
work-tree: .
```

### Environment variables

Any option might be overridden with an environment variable with the name the
same as an option with the prefix `GITLAB_CI_SEMVER_LABELS_` and an option name
with all capital letters with a dash character replaced with an underscore. Ie.:

```sh
GITLAB_CI_SEMVER_LABELS_FETCH_TAGS="false"
```

Additionally `$CI_PROJECT_ID` is used as a default `project` option value and
`$CI_SERVER_URL` as a default `gitlab-url` option value.

## CI

Example `.gitlab-ci.yml`:

```yaml
variables:
  DOCKER_IO: docker.io

stages:
  - semver
  - release

semver:validate:
  stage: semver
  rules:
    - if: $CI_MERGE_REQUEST_LABELS =~ /Release/ && $CI_MERGE_REQUEST_EVENT_TYPE == 'merge_train'
  image:
    name: $DOCKER_IO/dex4er/gitlab-ci-semver-labels
    entrypoint: [""]
  variables:
    GIT_DEPTH: 0
  script:
    - gitlab-ci-semver-labels --current || true
    - gitlab-ci-semver-label --fail

semver:bump:
  stage: semver
  rules:
    - if: $CI_COMMIT_BRANCH == $CI_DEFAULT_BRANCH && $CI_COMMIT_MESSAGE =~ /(^|\n)See merge request (\w[\w.+\/-]*)?!\d+/s
  image:
    name: $DOCKER_IO/dex4er/gitlab-ci-semver-labels
    entrypoint: [""]
  variables:
    GIT_DEPTH: 0
  script:
    - gitlab-ci-semver-labels --current || true
    - gitlab-ci-semver-labels --dotenv-file semver.env
  artifacts:
    reports:
      dotenv: semver.env

release:
  stage: release
  needs:
    - semver:bump
  rules:
    - if: $CI_COMMIT_BRANCH == $CI_DEFAULT_BRANCH && $CI_COMMIT_MESSAGE =~ /(^|\n)See merge request (\w[\w.+\/-]*)?!\d+/s
  image: registry.gitlab.com/gitlab-org/release-cli
  script:
    - echo "Release $version"
  release:
    tag_name: $version
    name: Release $version
    description: Automatic release by gitlab-ci-semver-labels
```
