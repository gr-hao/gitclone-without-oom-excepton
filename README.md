# gitclone-without-oom-excepton

This is POC for how to clone git repositories concurrency without OOM exception.

# Approach

- **Using `git clone --filter=blob:none <repo-url>` instead of using go-git clone API**

- **Clone dispatcher to manage clone concurrenncy**  
  By default app shall reserve 30 MiB of memory for each clone. Clone dispatcher (a module in this app) will try to get current free memory of the container to schedule next repository in the clone queue. All repositories need to be cloned must be go to clone-queue firstly, then clone dispatcher shall schedule clone depend on the current free memory. Clone dispatcher can schedule to clone multiple of repositories conccurency depend on the memory limit configured for the container.

- **Re-push repo to clone-queue if clone failed**  
  For each clone failed (shall not generating OOM exception), clone dispatcher will re-push it to the end of clone-queue for retry later when memory become free enough.

- **Variable guard-band memory**  
Each time a repository is failed to clone due to shortage of free memory, a guard-band memory is added more 30MiB (default value). The available memory now is recalculated by:  
  	`available_memory := current_free_memory_of_container - guard_band_memory`.  
Now the clone-dispatcher shall based on new value "available_memory" to decide schedule more repositories for next clones. The more clones failed, the more increasing of guard-band memory. This machanism to protect the clone dispatcher from trying to schedule continuously repo to clone, and having too many repositories cloning at a time.
Reversely, when a repository has been cloned successfully, the guard-band memory is substracted by 30MiB to encourage another repository has a chance to clone from the clone-queue.

<p align="center">
<img 
     src="https://github.com/gr-hao/gitclone-without-oom-excepton/blob/19cfc0be74a1ad229dd6bae00d5eb14457d6f1fe/diagram.png" 
     alt="" 
     title=""
     style="display: inline-block; margin: 0 auto; width: 550px">
</p> 

# How to run a test

- ## docker-compose.yml
  Change the `memory` limit and `MEMORY_LIMIT` to add more RAM for the container.

```
    deploy:
        resources:
          limits:
            memory: 300M
    environment:
      - REPO_FOLDER=/repos
      - MEMORY_LIMIT=300
```

- ## Run with many concurrency repos
  Add more repo URLs to below slice to clone more repositories.

```
	repoUrls := []string{
		"https://github.com/microsoft/vscode.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/googleapis/googleapis.git",
		"https://github.com/microsoft/vscode.git",
		"https://github.com/googleapis/googleapis.git",
		"https://github.com/flutter/flutter.git",
		"https://github.com/torvalds/linux.git",
		"https://github.com/flutter/flutter.git",
		"https://github.com/googleapis/googleapis.git",
		"https://github.com/flutter/flutter.git",
		"https://github.com/gcc-mirror/gcc.git",
		"https://github.com/googleapis/googleapis.git",
		"https://github.com/aosp-mirror/platform_development.git",
		"https://github.com/torvalds/linux.git",
	}
```
- ## Start the test  
`docker-compose up --build`
