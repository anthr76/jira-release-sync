# jira-release-sync

A GitHub Action that syncs GitHub Releases to Jira by creating a Jira Version and setting Fix Versions on linked issues.

## How it works

When a GitHub Release is published (e.g. by semantic-release), this action:

1. Finds the previous tag and compares commits between the two tags
2. Looks up merged PRs for each commit
3. Parses PR bodies for `jira: https://<domain>/browse/<KEY>-<NUMBER>` references
4. Creates a Jira Version (with the release changelog as its description)
5. Adds the version to each linked issue's Fix Versions field

## Usage

```yaml
on:
  release:
    types: [published]

permissions:
  contents: read
  pull-requests: read

jobs:
  sync:
    runs-on: ubuntu-latest
    steps:
      - uses: coreweave/jira-release-sync@v1
        with:
          jira_server: https://company.atlassian.net
          jira_project: PRJ
          jira_user: ${{ secrets.JIRA_USER }}
          jira_token: ${{ secrets.JIRA_TOKEN }}
```

## Inputs

| Input | Required | Default | Description |
|-------|----------|---------|-------------|
| `jira_server` | yes | — | Jira server URL |
| `jira_project` | yes | — | Jira project key (e.g. `PRJ`) |
| `jira_user` | yes | — | Jira API user email |
| `jira_token` | yes | — | Jira API token |
| `tag_format` | no | `""` | Regex with capture group to extract version from tag |
| `release_name_format` | no | `{version}` | Format string with `{version}` placeholder |

`GITHUB_TOKEN` is provided automatically by GitHub Actions.

### Tag format examples

For standard `v1.2.3` tags, no `tag_format` is needed — the `v` prefix is stripped automatically.

For monorepo tags like `myapp/v1.2.3`:

```yaml
with:
  tag_format: 'myapp/v(.+)'
```

### Release name format

Controls the Jira version name. Defaults to the bare version string.

```yaml
with:
  release_name_format: 'myapp {version}'  # creates "myapp 1.2.3"
```

## PR body format

Link Jira issues in PR bodies with:

```
jira: https://company.atlassian.net/browse/PRJ-123
```

Multiple references per PR are supported. The prefix is case-insensitive.
