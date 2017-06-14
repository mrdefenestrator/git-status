package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/fatih/color"
)

// RepoStatus x
type RepoStatus struct {
	Name         string
	RemoteBranch string
	Unpulled     int
	Unpushed     int
	Deltas       int
	ShouldReport bool
}

// Action x
type Action int

// x
const (
	None Action = iota
	Add
	Delete
	List
)

const storeName string = ".git-status"

var registered []string
var paths []string
var action Action
var store string
var showAll bool

func init() {
	var err error

	for _, arg := range os.Args[1:] {
		switch strings.ToUpper(arg) {
		case "-ADD":
			if action != None {
				fmt.Println("conflicting action flags provided")
				os.Exit(1)
			}
			action = Add
			break
		case "-DELETE":
			if action != None {
				fmt.Println("conflicting action flags provided")
				os.Exit(1)
			}
			action = Delete
			break
		case "-A":
			showAll = true
			break
		case "-LIST":
			if action != None {
				fmt.Println("conflicting action flags provided")
				os.Exit(1)
			}
			action = List
			break
		default:
			abs, err := filepath.Abs(arg)
			if err != nil {
				fmt.Println("error parsing path:", err.Error())
				os.Exit(1)
			}
			paths = append(paths, abs)
			break
		}
	}

	usr, err := user.Current()
	if err != nil {
		fmt.Println("error finding home dir:", err.Error())
		os.Exit(1)
	}
	store = path.Join(usr.HomeDir, storeName)
}

func main() {
	err := testGit()
	if err != nil {
		fmt.Println("git could not be found:", err.Error())
		os.Exit(1)
	}
	loadRegistered()
	switch action {
	case Add:
		registerPaths(paths)
		break
	case Delete:
		removePaths(paths)
		break
	case List:
		listRegistered()
		break
	default:
		getStatuses()
	}
}

func contains(array []string, target string) bool {
	for _, str := range array {
		if str == target {
			return true
		}
	}
	return false
}

func testGit() (err error) {
	git := exec.Command("git", "help")
	err = git.Run()
	return err
}

func loadRegistered() {
	raw, err := ioutil.ReadFile(store)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		fmt.Println("could not read registered repos")
		os.Exit(1)
	}
	registered = strings.Split(string(raw), "\n")
	if registered[len(registered)-1] == "" {
		registered = registered[0 : len(registered)-1]
	}
}

func listRegistered() {
	fmt.Println(len(registered), "path(s) registered:")
	for _, dir := range registered {
		fmt.Println("  " + dir)
	}
}

func removePaths(except []string) {
	os.Remove(store)
	f, err := os.OpenFile(store, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		fmt.Println("error saving paths:", err.Error())
		for _, path := range registered {
			fmt.Println(path)
		}
	}
	defer f.Close()
	for _, path := range registered {
		if !contains(except, path) {
			f.WriteString(path + "\n")
		}
	}
}

func registerPaths(targets []string) {
	f, err := os.OpenFile(store, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0666)
	if err != nil {
		fmt.Println("error registering path:", err.Error())
	}
	defer f.Close()
	for _, target := range targets {
		if contains(registered, target) {
			fmt.Println(target, "is already registered")
			continue
		}

		dotgitpath := path.Join(target, ".git")
		dotgit, err := os.Stat(dotgitpath)
		if err != nil || !dotgit.IsDir() {
			fmt.Println(target, "does not appear to be a git repo:", err.Error())
			continue
		}

		f.WriteString(target + "\n")
	}
}

func getStatuses() {
	repos := make([]RepoStatus, len(registered))
	nameWidth := 0
	branchWidth := 0
	for i, path := range registered {
		repo := getStatus(path)
		if len(repo.Name) > nameWidth {
			nameWidth = len(repo.Name)
		}
		if len(repo.RemoteBranch) > branchWidth {
			branchWidth = len(repo.RemoteBranch)
		}
		repos[i] = repo
	}
	nameWidth++
	for _, repo := range repos {
		if repo.ShouldReport {
			fmt.Printf("%-"+strconv.Itoa(nameWidth)+"s (%-"+strconv.Itoa(branchWidth)+"s) ", repo.Name, repo.RemoteBranch)
			cyan := color.New(color.FgCyan).PrintfFunc()
			yellow := color.New(color.FgYellow).PrintfFunc()
			if repo.Unpushed > 0 {
				cyan("↑%d ", repo.Unpushed)
			}
			if repo.Unpulled > 0 {
				cyan("↓%d ", repo.Unpulled)
			}
			if repo.Deltas > 0 {
				yellow("∆%d", repo.Deltas)
			}
			fmt.Println()
		} else if showAll {
			fmt.Printf("%-"+strconv.Itoa(nameWidth)+"s (%-"+strconv.Itoa(branchWidth)+"s) ", repo.Name, repo.RemoteBranch)
			color.Green("✔\n")
		}
	}
}

func getStatus(repo string) (status RepoStatus) {
	status.Name = getRepoName(repo)
	status.RemoteBranch = getRemote(repo)
	status.Unpulled, status.Unpushed = getPullPushCounts(repo, status.RemoteBranch)
	status.Deltas = getDeltas(repo)

	status.ShouldReport = status.Unpulled > 0 || status.Unpushed > 0 || status.Deltas > 0

	return status
}

func getRepoName(repo string) string {
	cmd := exec.Command("git", "config", "--get", "remote.origin.url")
	cmd.Dir = repo
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		fmt.Println("error getting repo name:", err.Error())
		return ""
	}
	remote := strings.TrimSpace(out.String())
	slash := strings.LastIndex(remote, "/")
	if slash != -1 {
		remote = remote[slash+1:]
	}
	remote = strings.TrimSuffix(remote, ".git")
	return remote
}

func getRemote(repo string) string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	cmd.Dir = repo
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		fmt.Println("error getting remote branch name:", err.Error())
		return ""
	}
	remote := strings.TrimSpace(out.String())
	return remote
}

func getPullPushCounts(repo string, remote string) (unpulled int, unpushed int) {
	cmd := exec.Command("git", "rev-list", "--count", "--left-right", remote+"..HEAD")
	cmd.Dir = repo
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		fmt.Println("error getting remote branch name:", err.Error())
		return -1, -1
	}
	nums := strings.TrimSpace(out.String())
	parts := strings.Split(nums, "\t")
	unpulled, err = strconv.Atoi(parts[0])
	if err != nil {
		fmt.Println("error reading unpulled commits:", err.Error())
		return -1, -1
	}
	unpushed, err = strconv.Atoi(parts[1])
	if err != nil {
		fmt.Println("error reading unpushed commits:", err.Error())
		return -1, -1
	}
	return unpulled, unpushed
}

func getDeltas(repo string) int {
	cmd := exec.Command("git", "status", "--porcelain=1")
	cmd.Dir = repo
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		fmt.Println("error getting remote branch name:", err.Error())
		return -1
	}
	raw := strings.TrimSpace(out.String())
	lines := strings.Split(raw, "\n")
	if len(lines) == 1 && lines[0] == "" {
		return 0
	}
	return len(lines)
}
