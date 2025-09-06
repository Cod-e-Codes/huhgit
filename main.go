package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/huh/spinner"
	"github.com/charmbracelet/lipgloss"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/google/go-github/v62/github"
)

type Action int

const (
	Cancel Action = iota
	Push
	Fork
	Skip
)

var highlight = lipgloss.NewStyle().Foreground(lipgloss.Color("#00D7D7"))
var commitTypes = []string{"fix", "feat", "docs", "style", "refactor", "test", "chore", "revert"}

func main() {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		log.Fatal("GITHUB_TOKEN env var is required")
	}

	repoPath, _ := os.Getwd()
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		log.Fatalf("Failed to open repo: %v", err)
	}

	remote, err := repo.Remote("origin")
	if err != nil {
		log.Fatalf("Failed to get origin remote: %v", err)
	}

	remoteURL := remote.Config().URLs[0]
	owner, name := parseGitHubURL(remoteURL)

	var action Action
	theme := huh.ThemeBase16()

	// Step 1 — Push/Fork menu
	pushForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[Action]().
				Value(&action).
				Options(
					huh.NewOption(fmt.Sprintf("%s/%s", owner, name), Push),
					huh.NewOption(fmt.Sprintf("Fork %s/%s", owner, name), Fork),
					huh.NewOption("Skip pushing branch", Skip),
					huh.NewOption("Cancel", Cancel),
				).
				Title("Where should we push the current branch?"),
		),
	).WithTheme(theme)

	if err := pushForm.Run(); err != nil {
		log.Fatal(err)
	}

	headRef, err := repo.Head()
	if err != nil {
		log.Fatalf("Failed to get HEAD: %v", err)
	}
	branchName := headRef.Name().Short()

	switch action {
	case Cancel:
		fmt.Println("Cancelled.")
		os.Exit(1)
	case Push:
		pushBranch(repo, token, fmt.Sprintf("%s/%s", owner, name), branchName)
	case Fork:
		fmt.Println("Pushing to fork…")
		pushBranch(repo, token, fmt.Sprintf("%s/%s-fork", owner, name), branchName)
	case Skip:
		fmt.Println("Skipping push…")
	}

	// Step 2 — PR form
	var prTitle, prBody string
	prForm := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("PR Title").Value(&prTitle).Placeholder("Title of your PR"),
			huh.NewText().Title("PR Body").Value(&prBody).Placeholder("Describe your changes"),
		),
	).WithTheme(theme)

	if err := prForm.Run(); err != nil {
		log.Fatal(err)
	}

	// Create PR
	sp := spinner.New().Title("Creating pull request...").Style(lipgloss.NewStyle().Foreground(lipgloss.Color("4")))
	_ = sp.Run()
	createPR(token, owner, name, prTitle, prBody, branchName)

	// Step 3 — Conventional Commit form
	var commitType, scope, summary, description string
	var confirm bool
	commitForm := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Type").Value(&commitType).Placeholder("feat").Suggestions(commitTypes),
			huh.NewInput().Title("Scope").Value(&scope).Placeholder("scope"),
		),
		huh.NewGroup(
			huh.NewInput().Title("Summary").Value(&summary).Placeholder("Summary of changes"),
			huh.NewText().Title("Description").Value(&description).Placeholder("Detailed description of changes"),
		),
		huh.NewGroup(
			huh.NewConfirm().Title("Commit changes?").Value(&confirm),
		),
	).WithTheme(theme)

	if err := commitForm.Run(); err != nil {
		log.Fatal(err)
	}

	if confirm {
		msg := fmt.Sprintf("%s(%s): %s\n\n%s", commitType, scope, summary, description)
		fmt.Println(highlight.Render("Final commit message:"))
		fmt.Println(msg)
	}
}

func pushBranch(repo *git.Repository, token, fullName, branch string) {
	auth := &http.BasicAuth{
		Username: "git",
		Password: token,
	}

	fmt.Printf("Pushing branch %s to %s...\n", branch, fullName)
	err := repo.Push(&git.PushOptions{
		Auth: auth,
		RefSpecs: []config.RefSpec{
			config.RefSpec(fmt.Sprintf("refs/heads/%[1]s:refs/heads/%[1]s", branch)),
		},
		RemoteName: "origin",
		Progress:   os.Stdout,
	})
	if err != nil {
		if err == git.NoErrAlreadyUpToDate {
			fmt.Println("Branch already up-to-date.")
			return
		}
		log.Fatalf("Push failed: %v", err)
	}
	fmt.Println("Push successful!")
}

func createPR(token, owner, repo, title, body, branch string) {
	ctx := context.Background()
	client := github.NewTokenClient(ctx, token)

	newPR := &github.NewPullRequest{
		Title:               github.String(title),
		Head:                github.String(branch),
		Base:                github.String("main"),
		Body:                github.String(body),
		MaintainerCanModify: github.Bool(true),
	}

	pr, _, err := client.PullRequests.Create(ctx, owner, repo, newPR)
	if err != nil {
		log.Fatalf("Failed to create PR: %v", err)
	}
	fmt.Printf("Pull request created: %s\n", pr.GetHTMLURL())
}

func parseGitHubURL(url string) (owner, name string) {
	url = strings.TrimSuffix(url, ".git")
	if strings.HasPrefix(url, "git@") {
		parts := strings.Split(strings.TrimPrefix(url, "git@github.com:"), "/")
		return parts[0], parts[1]
	} else if strings.HasPrefix(url, "https://") {
		parts := strings.Split(strings.TrimPrefix(url, "https://github.com/"), "/")
		return parts[0], parts[1]
	}
	return "", ""
}
// test change
// test change
