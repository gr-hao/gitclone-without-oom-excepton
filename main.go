package main

import (
	"fmt"
	"gitclone/github"
	"sync"
	"time"
)

func main() {
	fmt.Println("Start to git test")

	/* repoUrls := []string{
		"https://github.com/microsoft/vscode.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/torvalds/linux.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/torvalds/linux.git",
	} */

	/* repoUrls := []string{
		"https://github.com/microsoft/vscode.git",
	} */

	bigRepos := []string{
		"https://github.com/torvalds/linux.git",
		"https://github.com/gcc-mirror/gcc.git",
		"https://github.com/torvalds/linux.git",
		"https://github.com/gcc-mirror/gcc.git",
		"https://github.com/torvalds/linux.git",
		"https://github.com/gcc-mirror/gcc.git",
		"https://github.com/torvalds/linux.git",
		"https://github.com/gcc-mirror/gcc.git",
		"https://github.com/torvalds/linux.git",
		"https://github.com/gcc-mirror/gcc.git",
		"https://github.com/torvalds/linux.git",
		"https://github.com/gcc-mirror/gcc.git",
	}

	midRepos := []string{
		"https://github.com/microsoft/vscode.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/microsoft/vscode.git",
	}

	smallRepos := []string{
		"https://github.com/googleapis/googleapis.git",
		"https://github.com/aosp-mirror/platform_development.git",
		"https://github.com/flutter/flutter.git",
		"https://github.com/googleapis/googleapis.git",
		"https://github.com/aosp-mirror/platform_development.git",
		"https://github.com/flutter/flutter.git",
		"https://github.com/googleapis/googleapis.git",
		"https://github.com/aosp-mirror/platform_development.git",
		"https://github.com/flutter/flutter.git",
		"https://github.com/googleapis/googleapis.git",
		"https://github.com/aosp-mirror/platform_development.git",
		"https://github.com/flutter/flutter.git",
		"https://github.com/googleapis/googleapis.git",
	}

	g := github.NewGit()
	_ = g

	defer func() {
		r := recover()
		fmt.Println(r)
	}()

	go func() {
		//g.BloblessClone("https://github.com/gcc-mirror/gcc.git")
		//return
		for {
			if g.GitRepoNums() <= 1 {
				for _, r := range bigRepos {
					g.BloblessClone(r)
				}
			}
			if g.GitRepoNums() <= len(bigRepos) {
				for _, r := range midRepos {
					g.BloblessClone(r)
				}
				for _, r := range smallRepos {
					g.BloblessClone(r)
				}
			}
			time.Sleep(5 * time.Second)
		}
	}()

	var wg sync.WaitGroup
	wg.Add(1)
	wg.Wait()
}
