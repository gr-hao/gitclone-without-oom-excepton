package github

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/mackerelio/go-osstat/memory"
	"github.com/shirou/gopsutil/mem"
)

var (
	RepoSize   map[string]int = map[string]int{}
	sizeMemMap                = map[int][]int{
		20:  {0, 100},
		40:  {100, 300},
		60:  {300, 500},
		70:  {500, 1024},
		100: {1024, 2048},
		140: {2048, 1024 * 3},
		180: {1024 * 3, 1024 * 4},
		250: {1024 * 4, 1024 * 5},
	}
)

const (
	QUEUED   = "queued"
	CLONING  = "cloning"
	STOPED   = "stoped"
	FINISHED = "finised"
)

type Repo struct {
	RepoUrl            string
	cloneType          string
	RepoName           string
	RepoDirPath        string
	RemoteBranches     []string
	minMemmoryRequired int
	memmoryRequired    int
	failureCount       int
	cloneAt            int64
	repo               *git.Repository
	worktree           *git.Worktree
	cmd                *exec.Cmd
	cloneStatus        string
	err                error
}

func (r *Repo) getRemoteBranches() ([]string, error) {
	var err error
	r.RemoteBranches = []string{}

	if r.repo == nil {
		r.repo, err = git.PlainOpen(r.RepoDirPath)
		if err != nil {
			return nil, err
		}
	}

	r.worktree, err = r.repo.Worktree()
	if err != nil {
		fmt.Println("Build WorkTree failed!", err)
		return nil, err
	}

	fmt.Println("Build WorkTree successfully!", r.RepoUrl)

	remote, err := r.repo.Remote("origin")
	if err != nil {
		return nil, err

	}
	refList, err := remote.List(&git.ListOptions{})
	if err != nil {
		return nil, err
	}

	refPrefix := "refs/heads/"
	for _, ref := range refList {
		refName := ref.Name().String()
		if !strings.HasPrefix(refName, refPrefix) {
			continue
		}
		branchName := refName[len(refPrefix):]
		r.RemoteBranches = append(r.RemoteBranches, branchName)
	}
	return r.RemoteBranches, nil
}

func (r *Repo) Checkout(url, branch, commit, dest string) error {
	var err error
	repo := r.repo

	if repo == nil {
		repo, err = git.PlainOpen(r.RepoDirPath)
		if err != nil {
			return err
		}
	}

	w, err := repo.Worktree()
	if err != nil {
		return err
	}
	err = w.Checkout(&git.CheckoutOptions{
		Hash: plumbing.NewHash(commit),
	})
	if err != nil {
		return err
	}
	return nil
}

type Git struct {
	repoFolder            string
	ramAtStart2           uint64
	ramAtStart1           uint64
	ramCofigured          int
	minMemoryForEachClone int
	totalMemoryConsuming  int
	protectionGuard       int
	memGuard              int
	readyQueueChan        chan *Repo
	runningQueueChan      chan *Repo
	pressureTimeChan      chan bool
	successChan           chan int
	readyRepoQueue        []*Repo
	repoClonings          []*Repo
	cloneSuccess          uint64
	pressureTime          bool
	lock                  sync.RWMutex
}

func NewGit() *Git {
	repoFolder := os.Getenv("REPO_FOLDER")
	if repoFolder == "" {
		fmt.Println("REPO_FOLDER has not set, use local ./repos!")
		repoFolder = "./repos"
	}

	g := Git{
		repoFolder:     repoFolder,
		readyRepoQueue: []*Repo{},
		readyQueueChan: make(chan *Repo, 5),
	}

	g.minMemoryForEachClone = 30
	ramCofigured := os.Getenv("MEMORY_LIMIT")

	x, _ := strconv.ParseInt(ramCofigured, 10, 64)
	if x == 0 {
		g.ramCofigured = 80
	} else {
		g.ramCofigured = int(x)
	}
	fmt.Printf("MEMORY_LIMIT: %d\n", g.ramCofigured)

	g.protectionGuard = 70
	mg := os.Getenv("MEMORY_GUARD")
	x, _ = strconv.ParseInt(mg, 10, 64)
	if x > 0 {
		g.protectionGuard = int(x)
	}
	fmt.Printf("MEMORY_PROTECTION_GUARD: %d\n", g.protectionGuard)

	cmd := fmt.Sprintf("rm -rf %s/gitclone*", repoFolder)
	ShellRun(cmd, nil)

	go g.GitDispatcher()
	return &g
}

func (g *Git) GitRepoNums() int {
	g.lock.RLock()
	defer g.lock.RUnlock()
	return len(g.repoClonings) + len(g.readyRepoQueue)
}

func (g *Git) CloningNum() int {
	g.lock.RLock()
	defer g.lock.RUnlock()
	return len(g.repoClonings)
}
func (g *Git) WaitingNum() int {
	g.lock.RLock()
	defer g.lock.RUnlock()
	return len(g.readyRepoQueue)
}

func (g *Git) GitDispatcher() {
	g.successChan = make(chan int, 5)
	g.runningQueueChan = make(chan *Repo, 5)
	g.pressureTimeChan = make(chan bool)
	g.cloneSuccess = 0
	g.pressureTime = false

	for {
		select {
		case x := <-g.successChan:
			g.cloneSuccess += uint64(x)
		case p := <-g.pressureTimeChan:
			g.pressureTime = p
		case repo := <-g.readyQueueChan:
			repo.cloneStatus = QUEUED
			g.readyRepoQueue = append(g.readyRepoQueue, repo)
		case repo := <-g.runningQueueChan:
			if repo.cloneStatus == FINISHED {
				for i, r := range g.repoClonings {
					if r.RepoDirPath == repo.RepoDirPath && repo.cmd == nil {
						fmt.Printf("*** Repo %s stoped cloning!\n", r.RepoName)
						g.repoClonings = append(g.repoClonings[0:i], g.repoClonings[i+1:]...)
						break
					}
				}
				g.cloneFinished(repo)
			} else if repo.cloneStatus == CLONING {
				g.repoClonings = append(g.repoClonings, repo)
			}
		case <-time.After(1 * time.Second):
			// Try to schedule for every 1 second
			g.scheduleToClone()
		}
	}
}

func (g *Git) scheduleToClone() {
	free, used := g.GetMemoryUsage()
	clonings := g.CloningNum()

	// Sort cloning queue by clone-time: youngest -> oldest
	// At pressure time, stop the youngest cloning to save memory.
	sort.Slice(g.repoClonings, func(i, j int) bool {
		if g.repoClonings[i].cloneAt > g.repoClonings[j].cloneAt {
			return true
		}
		return false
	})

	if used+g.protectionGuard > g.ramCofigured && len(g.repoClonings) >= 1 {
		g.pressureTime = true
		for _, r := range g.repoClonings {
			if r.cloneStatus != STOPED && r.cmd != nil && r.cmd.Process != nil {
				// Stop the youngest clonning to save memory for others
				fmt.Printf("\n\n\n\n****** Stop cloning '%s/%s' because memory shortage ******\n\n\n\n", r.RepoDirPath, r.RepoName)
				//id, err := syscall.Getpgid(r.cmd.Process.Pid)
				//syscall.Kill(-r.cmd.Process.Pid, syscall.SIGKILL)
				r.cmd.Process.Kill()
				r.cloneStatus = STOPED
				break
			}
		}
	} else if used+int(float64(g.protectionGuard)*1.4) <= g.ramCofigured {
		time.AfterFunc(time.Second*20, func() {
			// Wait after 20 seconds, then turn of pressure time flag
			g.pressureTimeChan <- false
		})
	}

	fmt.Printf("cloned success: %d, queue: %d, clonings: %d,  MemUsed=%d Mi/%d Mi, PressureTime: %v\n",
		g.cloneSuccess, len(g.readyRepoQueue), clonings, used, g.ramCofigured, g.pressureTime)

	// Print all repos that's clonning
	fmt.Printf("%d Clones: [\n", clonings)
	for _, r := range g.repoClonings {
		if r.cloneStatus == STOPED {
			fmt.Printf("\t%s/%s --> Require %v (MiB) (status: 'PENDING' - waiting for free mem... )\n", r.RepoDirPath, r.RepoName, r.memmoryRequired)
		} else {
			fmt.Printf("\t%s/%s --> Require %v (MiB) (status: %s)\n", r.RepoDirPath, r.RepoName, r.memmoryRequired, r.cloneStatus)
		}
	}
	fmt.Printf("]\n")

	// No more clonning if in pressure time
	if len(g.readyRepoQueue) == 0 || g.pressureTime {
		return
	}

	// Repo with small required-mem is higher priority to run fist
	// Should be optimized with order queue.
	sort.Slice(g.readyRepoQueue, func(i, j int) bool {
		if g.readyRepoQueue[i].memmoryRequired < g.readyRepoQueue[j].memmoryRequired {
			return true
		}
		return false
	})

	for i, repo := range g.readyRepoQueue {
		totalNextMemUsing := 0

		if clonings == 0 {
			// If no repo is cloning, then make the best memory condition for this repo.
			g.memGuard = 0
			g.totalMemoryConsuming = 0
			repo.memmoryRequired = repo.minMemmoryRequired
		} else {
			totalNextMemUsing = g.totalMemoryConsuming + repo.memmoryRequired
		}

		available := g.ramCofigured - g.memGuard

		if repo.failureCount >= 20 {
			fmt.Printf("Too many failures to clone repo %s/%s (require-mem: %d). Skip this!\n",
				repo.RepoDirPath, repo.RepoName, repo.memmoryRequired)
			g.readyRepoQueue = append(g.readyRepoQueue[0:i], g.readyRepoQueue[i+1:]...)
			break
		}

		if (totalNextMemUsing <= available && free >= repo.memmoryRequired) || (clonings == 0) {
			fmt.Printf("totalNextMemUsing: %d, available: %d, free: %d, memGuard: %d, memmoryRequired: %d\n",
				totalNextMemUsing, available, free, g.memGuard, repo.memmoryRequired)

			g.totalMemoryConsuming += repo.memmoryRequired
			g.readyRepoQueue = append(g.readyRepoQueue[0:i], g.readyRepoQueue[i+1:]...)
			go g.startClone(repo)
			break // Do not remove this line
		}
	}
}

func (g *Git) startClone(repo *Repo) {
	fmt.Printf("Start clone '%s' ...\n", repo.RepoUrl)

	repo.cloneStatus = CLONING
	repo.cloneAt = time.Now().Unix()
	g.runningQueueChan <- repo

	repo.RepoDirPath = fmt.Sprintf("%s/gitclone-%s", g.repoFolder, RandStr(8))
	repo.err = nil
	if repo.cloneType == "blobless" {
		repo.err = g.doBloblessClone(repo, true)
	} else {
		repo.err = g.doFullClone(repo)
	}

	repo.cmd = nil
	repo.cloneStatus = FINISHED
	g.runningQueueChan <- repo
}

func (g *Git) cloneFinished(repo *Repo) {
	// Clone finished, reduce total memory usage (just an estimation)
	if g.totalMemoryConsuming >= repo.memmoryRequired {
		g.totalMemoryConsuming -= repo.memmoryRequired
	}

	if repo.err != nil {
		repo.failureCount += 1

		if repo.memmoryRequired < g.ramCofigured {
			if g.memGuard > repo.memmoryRequired {
				g.memGuard -= repo.memmoryRequired
			}
			x := float64(repo.memmoryRequired) * 1.13
			repo.memmoryRequired = int(x)
			fmt.Printf("Next memmory-required for repo %s:  %d MiB.\n", repo.RepoName, repo.memmoryRequired)

			g.memGuard += repo.memmoryRequired
		}

		fmt.Printf("Failed to clone '%s', requeue!\n", repo.RepoUrl)
		g.readyQueueChan <- repo
	} else {
		if g.memGuard >= repo.memmoryRequired {
			g.memGuard -= repo.memmoryRequired
		}

		repo.failureCount = 0
		g.successChan <- 1
		fmt.Printf("=== Clone '%s' successfully ===\n", repo.RepoUrl)
	}
}

func (g *Git) FullClone(RepoUrl string) {
	repo := Repo{
		RepoUrl:            RepoUrl,
		cloneType:          "full",
		memmoryRequired:    g.minMemoryForEachClone,
		minMemmoryRequired: g.minMemoryForEachClone,
	}
	g.readyQueueChan <- &repo
}

func (g *Git) BloblessClone(RepoUrl string) {
	minMemoryForEachClone := 400

	repoSize, rname := g.GetRepoSize(RepoUrl)
	if repoSize == 0 {
		return
	}
	for s, r := range sizeMemMap {
		if r[0] <= repoSize && int(repoSize) < r[1] {
			minMemoryForEachClone = s
			break
		}
	}

	repo := Repo{
		RepoUrl:            RepoUrl,
		RepoName:           rname,
		cloneType:          "blobless",
		memmoryRequired:    minMemoryForEachClone,
		minMemmoryRequired: minMemoryForEachClone,
	}
	fmt.Printf("Queue to clone: %s, required-mem: %d\n", RepoUrl, minMemoryForEachClone)
	g.readyQueueChan <- &repo
}

func (g *Git) doFullClone(repo *Repo) error {
	repo.RepoDirPath = fmt.Sprintf("%s/%s", g.repoFolder, RandStr(8))
	r, err := git.PlainOpen(repo.RepoDirPath)
	if err != nil {
		r, err = git.PlainClone(
			repo.RepoDirPath,
			false,
			&git.CloneOptions{
				URL:      repo.RepoUrl,
				Progress: os.Stdout,
				Depth:    0,
			})
		if err != nil {
			return err
		}
		workTree, err := r.Worktree()
		_ = workTree
		if err != nil {
			return err
		}
		fmt.Printf("Cloned repo %s successfully!\n", repo.RepoUrl)
	} else {
		workTree, err := r.Worktree()
		if err != nil {
			return err
		}
		err = workTree.Pull(&git.PullOptions{RemoteName: "origin"})
		if err != nil {
			fmt.Printf("Repo %s: %s\n", repo.RepoUrl, err.Error())
		}
		fmt.Printf("Pull repo %s successfully!\n", repo.RepoUrl)
	}
	repo.repo = r
	repo.getRemoteBranches()
	return nil
}

func (g *Git) doBloblessClone(repo *Repo, debug bool) error {
	//cloneCmd := fmt.Sprintf("git clone -j1 --filter=blob:none %s %s", repo.RepoUrl, repo.RepoDirPath)
	cloneCmd := fmt.Sprintf("git clone -j3 --filter=blob:none %s %s", repo.RepoUrl, repo.RepoDirPath)
	removeCmd := fmt.Sprintf("rm -rf %s", repo.RepoDirPath)

	//cmd := fmt.Sprintf("git clone --filter=tree:0 %s %s", repo.RepoUrl, repo.RepoDirPath)
	fmt.Printf("Runing: %s\n", cloneCmd)

	repo.cmd = nil
	err := ShellRun(cloneCmd, func(c *exec.Cmd) {
		repo.cmd = c
	})
	if err != nil {
		fmt.Println(err)
		repo.cmd = nil
		ShellRun(removeCmd, nil)
		return err
	}

	//_, err = repo.getRemoteBranches()

	ShellRun(removeCmd, nil)
	return nil
}

func (g *Git) GetRepoSize(repoUrl string) (int, string) {
	s := strings.Split(repoUrl, "github.com/")
	if len(s) <= 1 {
		return 0, ""
	}

	rname := strings.ReplaceAll(s[1], ".git", "")
	//url := "https://api.github.com/repos/" + r
	url := fmt.Sprintf("https://gr-hao:xxx@api.github.com/repos/%s", rname)

	x, ok := RepoSize[repoUrl]
	if ok {
		return x, rname
	}

	resp, err := http.Get(url)
	if err != nil {
		return 0, ""
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, ""
	}

	var j map[string]interface{}
	err = json.Unmarshal(body, &j)
	if err != nil {
		return 0, ""
	}

	size, ok := j["size"]
	if !ok {
		return 0, ""
	}
	ss := int(size.(float64) / 1024)
	RepoSize[repoUrl] = ss
	return ss, rname
}

func (g *Git) GetMemoryUsage() (int, int) {
	if g.ramAtStart1 == 0 {
		s, err := memory.Get()
		if err != nil {
			fmt.Println(err)
		} else {
			g.ramAtStart1 = s.Used
		}
	}

	if g.ramAtStart2 == 0 {
		memInfo, _ := mem.VirtualMemory()
		g.ramAtStart2 = memInfo.Used
	}

	used := 0
	s, _ := memory.Get()
	if s.Used > g.ramAtStart1 {
		used = int(BToMb(s.Used - g.ramAtStart1))
	}

	if true {
		_used := 0
		memInfo, _ := mem.VirtualMemory()
		if memInfo.Used > g.ramAtStart2 {
			_used = int(BToMb(memInfo.Used - g.ramAtStart2))
		}
		fmt.Printf("[*******************] Used1: %d, Used2: %d\n", used, _used)
	}

	if used == 0 {
		memInfo, _ := mem.VirtualMemory()
		if memInfo.Used > g.ramAtStart2 {
			used = int(BToMb(memInfo.Used - g.ramAtStart2))
		}
		fmt.Printf("[*********] memInfo.Used: %d, g.ramAtStart2: %d, Used: %d\n", BToMb(memInfo.Used), BToMb(g.ramAtStart2), used)
	}

	// Used number only reflect the memory consumed by git clones,
	// not for the whole memory that contiainer app is being used.
	return int(g.ramCofigured - used), int(used)
}
