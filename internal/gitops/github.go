package gitops

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"path"
	"slices"

	"github.com/google/go-github/v93/github"
)

// GitHubClient wraps a github.Client with extra fields for owner, repoName and baseBranch
type GitHubClient struct {
	client     *github.Client
	owner      string
	repoName   string
	baseBranch string
}

// NewGitHubClient creates a new GitHubClient object
func NewGitHubClient(token, owner, repo, baseBranch string) (*GitHubClient, error) {
	ghClient, err := github.NewClient(github.WithAuthToken(token))
	if err != nil {
		return nil, fmt.Errorf("creating github client: %w", err)
	}
	return &GitHubClient{
		client:     ghClient,
		owner:      owner,
		repoName:   repo,
		baseBranch: baseBranch,
	}, nil
}

// OpenPR builds and opens a GitHub pull request from provided configs
// Opens a single PR on a new branch from the base branch, and returns the PR URL
func (c *GitHubClient) OpenPR(ctx context.Context, branch, title, body string, files map[string][]byte) (string, error) {
	baseRef, _, err := c.client.Git.GetRef(
		ctx,
		c.owner,
		c.repoName,
		"heads/"+c.baseBranch,
	)
	if err != nil {
		return "", fmt.Errorf("getting base ref: %w", err)
	}
	baseSHA := baseRef.GetObject().GetSHA()

	baseCommit, _, err := c.client.Git.GetCommit(
		ctx,
		c.owner,
		c.repoName,
		baseSHA,
	)
	if err != nil {
		return "", fmt.Errorf("getting base commit: %w", err)
	}

	baseTreeSHA := baseCommit.GetTree().GetSHA()

	treeEntries := make([]*github.TreeEntry, 0, len(files))
	for _, p := range slices.Sorted(maps.Keys(files)) {
		content := files[p]
		treeEntries = append(treeEntries, &github.TreeEntry{
			Path:    new(p),
			Mode:    new("100644"),
			Type:    new("blob"),
			Content: new(string(content)),
		})
	}

	ghTree, _, err := c.client.Git.CreateTree(
		ctx,
		c.owner,
		c.repoName,
		baseTreeSHA,
		treeEntries,
	)
	if err != nil {
		return "", fmt.Errorf("creating tree: %w", err)
	}

	createCommit := github.Commit{
		Message: new(title),
		Tree:    ghTree,
		Parents: []*github.Commit{{SHA: new(baseSHA)}},
	}

	newCommit, _, err := c.client.Git.CreateCommit(
		ctx,
		c.owner,
		c.repoName,
		createCommit,
		nil,
	)
	if err != nil {
		return "", fmt.Errorf("generating new commit: %w", err)
	}

	branchRef := github.CreateRef{
		Ref: "refs/heads/" + branch,
		SHA: newCommit.GetSHA(),
	}

	_, _, err = c.client.Git.CreateRef(
		ctx,
		c.owner,
		c.repoName,
		branchRef,
	)
	if err != nil {
		return "", fmt.Errorf("creating branch %q: %w", branch, err)
	}

	prRef := github.CreatePullRequest{
		Title: new(title),
		Head:  branch,
		Base:  c.baseBranch,
		Body:  new(body),
	}

	pr, _, err := c.client.PullRequests.Create(
		ctx,
		c.owner,
		c.repoName,
		prRef,
	)
	if err != nil {
		return "", fmt.Errorf("branch %q was created but opening pull request failed: %w", branch, err)
	}
	return pr.GetHTMLURL(), nil
}

// ServiceExists checks whether or not the ForgeService already exists in the gitops repo
func (c *GitHubClient) ServiceExists(ctx context.Context, name string) (bool, error) {
	svcPath := path.Join("services", name)

	opts := &github.RepositoryContentGetOptions{
		Ref: c.baseBranch,
	}

	_, _, repoResp, err := c.client.Repositories.GetContents(
		ctx,
		c.owner,
		c.repoName,
		svcPath,
		opts,
	)
	if err == nil {
		return true, nil
	}
	if repoResp != nil && repoResp.StatusCode == http.StatusNotFound {
		return false, nil
	}

	return false, fmt.Errorf("verifying service  %q exists: %w", name, err)
}

// BranchExists checks whether or not the requested branch already exists in the gitops repo
func (c *GitHubClient) BranchExists(ctx context.Context, branch string) (bool, error) {
	branchPath := path.Join("heads", branch)

	_, branchResp, err := c.client.Git.GetRef(ctx, c.owner, c.repoName, branchPath)
	if err == nil {
		return true, nil
	}
	if branchResp != nil && branchResp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	return false, fmt.Errorf("verifying branch %q exists: %w", branchPath, err)
}
