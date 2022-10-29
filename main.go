package main

import (
	"fmt"
	"gitclone/github"
	"sync"
	"time"
)

func main() {
	fmt.Println("Start git test")
	
	fmt.Println("test 1") 

	bigRepos := []string{
		"https://github.com/torvalds/linux.git",
		"https://github.com/aosp-mirror/platform_frameworks_base.git",
		"https://github.com/gcc-mirror/gcc.git",
		"https://github.com/torvalds/linux.git",
		"https://github.com/urho3d/android-ndk.git",
		"https://github.com/gcc-mirror/gcc.git",
		"https://github.com/aosp-mirror/platform_frameworks_base.git",
		"https://github.com/torvalds/linux.git",
		"https://github.com/gcc-mirror/gcc.git",
		"https://github.com/urho3d/android-ndk.git",
		"https://github.com/torvalds/linux.git",
		"https://github.com/aosp-mirror/platform_frameworks_base.git",
		"https://github.com/gcc-mirror/gcc.git",
		"https://github.com/gcc-mirror/gcc.git",
		"https://github.com/urho3d/android-ndk.git",
		"https://github.com/torvalds/linux.git",
		"https://github.com/aosp-mirror/platform_frameworks_base.git",
		"https://github.com/gcc-mirror/gcc.git",
		"https://github.com/gcc-mirror/gcc.git",
		"https://github.com/urho3d/android-ndk.git",
		"https://github.com/apple/swift.git",
	}

	midRepos := []string{
		"https://github.com/microsoft/vscode.git",
		"https://github.com/facebook/react-native.git",
		"https://github.com/tensorflow/tensorflow.git",
		"https://github.com/mozilla-mobile/firefox-ios.git",
		"https://github.com/gradle/gradle.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/facebook/react-native.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/isl-org/Open3D.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/facebook/react-native.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/facebook/react-native.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/facebook/react-native.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/isl-org/Open3D.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/facebook/react-native.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/gradle/gradle.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/facebook/react-native.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/facebook/react-native.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/facebook/react-native.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/facebook/react-native.git",
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

	/* repos := []string{
		"https://github.com/apple/swift.git",
		"https://github.com/isl-org/Open3D.git",
	}

	for _, r := range repos {
		repoSize, name := g.GetRepoSize(r)
		fmt.Printf("%s repoSize = %d\n", name, repoSize)
	}
	return */

	go func() {
		//g.BloblessClone("https://github.com/gcc-mirror/gcc.git")
		//return
		for {
			if g.GitRepoNums() <= 1 {
				for _, r := range bigRepos {
					g.BloblessClone(r)
				}
				for _, r := range midRepos {
					g.BloblessClone(r)
				}
				for _, r := range smallRepos {
					g.BloblessClone(r)
				}
			}
			time.Sleep(10 * time.Second)
		}
	}()

	var wg sync.WaitGroup
	wg.Add(1)
	wg.Wait()
}
