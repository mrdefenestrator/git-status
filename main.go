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
	Help
)

const storeName string = ".git-status"

var registered []string
var paths []string
var action Action
var store string
var showAll bool

func init() {
	var err error

	if len(os.Args) >= 2 {
		switch strings.ToUpper(os.Args[1]) {
		case "+":
			fallthrough
		case "-ADD":
			action = Add
		case "-":
			fallthrough
		case "-D":
			fallthrough
		case "-DEL":
			fallthrough
		case "-DELETE":
			fallthrough
		case "-R":
			fallthrough
		case "-REMOVE":
			action = Delete
		case "-L":
			fallthrough
		case "-LS":
			fallthrough
		case "-LIST":
			action = List
		case "-A":
			fallthrough
		case "-ALL":
			showAll = true
		case "-H":
			fallthrough
		case "-HELP":
			fallthrough
		case "/?":
			action = Help
		}
	}
	if len(os.Args) >= 3 && (action == Add || action == Delete) {
		for _, arg := range os.Args[2:] {
			abs, err := filepath.Abs(arg)
			if err != nil {
				fmt.Println("error parsing path:", err.Error())
				os.Exit(1)
			}
			paths = append(paths, abs)
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
	_, err := exec.LookPath("git")
	if err != nil {
		fmt.Println("git could not be found:", err.Error())
		os.Exit(1)
	}
	loadRegistered()
	switch action {
	case Add:
		registerPaths(paths)
	case Delete:
		removePaths(paths)
	case List:
		listRegistered()
	case Help:
		printUsage()
	default:
		getStatuses()
	}
}

func printUsage() {
	usage := `git-status [-add|-delete paths...]|[-list|-a|-h]
  -add     Add a folder to monitor
  -delete  Remove a folder, stop monitoring
  -list    List all monitored paths
  -a       Show status on all registered paths
  -h       Show this help`
	fmt.Println(usage)
}

func contains(array []string, target string) bool {
	for _, str := range array {
		if str == target {
			return true
		}
	}
	return false
}

func padRight(str string, length int) string {
	for len(str) < length {
		str += " "
	}
	return str
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

		if !isRepo(target) {
			fmt.Println(target, "does not appear to be a git repo")
			continue
		}

		f.WriteString(target + "\n")
	}
}

func isRepo(dir string) bool {
	dotgitpath := path.Join(dir, ".git")
	dotgit, err := os.Stat(dotgitpath)
	if err != nil || !dotgit.IsDir() {
		return false
	}
	return true
}

func getStatuses() {
	repos := make([]RepoStatus, len(registered))
	nameWidth := 0
	branchWidth := 0
	for i, path := range registered {
		if !isRepo(path) {
			fmt.Println(path, "no longer appears to be a git repo, unregistering")
			removePaths([]string{path})
		}
		repo := getStatus(path)
		if (repo.ShouldReport || showAll) && len(repo.Name) > nameWidth {
			nameWidth = len(repo.Name)
		}
		if (repo.ShouldReport || showAll) && len(repo.RemoteBranch) > branchWidth {
			branchWidth = len(repo.RemoteBranch)
		}
		repos[i] = repo
	}
	for _, repo := range repos {
		if repo.ShouldReport {
			fmt.Printf("%s (%s) ", padRight(repo.Name, nameWidth), padRight(repo.RemoteBranch, branchWidth))
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
	status.Unpulled = getUnpulled(repo, status.RemoteBranch)
	status.Unpushed = getUnpushed(repo, status.RemoteBranch)
	status.Deltas = getDeltas(repo)

	status.ShouldReport = status.Unpulled > 0 || status.Unpushed > 0 || status.Deltas > 0

	return status
}

func getRepoName(repo string) string {
	remote, err := getCmdOutput(repo, "git", "config", "--get", "remote.origin.url")
	if err != nil {
		fmt.Println("error getting repo name:", err.Error())
		return ""
	}
	slash := strings.LastIndex(remote, "/")
	if slash != -1 {
		remote = remote[slash+1:]
	}
	remote = strings.TrimSuffix(remote, ".git")
	return remote
}

func getRemote(repo string) string {
	raw, err := getCmdOutput(repo, "git", "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	if err != nil {
		fmt.Println("error getting remote branch name:", err.Error())
		return ""
	}
	return raw
}

func getUnpulled(repo string, remote string) (unpulled int) {
	raw, err := getCmdOutput(repo, "git", "rev-list", "--count", "HEAD.."+remote)
	if err != nil {
		fmt.Println("error getting remote branch name:", err.Error())
		return -1
	}
	unpulled, err = strconv.Atoi(raw)
	return unpulled
}

func getUnpushed(repo string, remote string) (unpushed int) {
	raw, err := getCmdOutput(repo, "git", "rev-list", "--count", remote+"..HEAD")
	if err != nil {
		fmt.Println("error getting remote branch name:", err.Error())
		return -1
	}
	unpushed, err = strconv.Atoi(raw)
	return unpushed
}

func getDeltas(repo string) int {
	raw, err := getCmdOutput(repo, "git", "status", "--porcelain=1")
	if err != nil {
		fmt.Println("error getting remote branch name:", err.Error())
		return -1
	}
	lines := strings.Split(raw, "\n")
	if len(lines) == 1 && lines[0] == "" {
		return 0
	}
	return len(lines)
}

func getCmdOutput(workingDir string, name string, arg ...string) (string, error) {
	cmd := exec.Command(name, arg...)
	cmd.Dir = workingDir
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	raw := strings.TrimSpace(out.String())
	return raw, nil
}
