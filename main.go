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
	Name              string
	RemoteBranch      string
	RemoteBranchError bool
	Unpulled          int
	Unpushed          int
	Deltas            int
	ShouldReport      bool
}

// Action x
type Action int

// x
const (
	ActionNone Action = iota
	ActionAdd
	ActionDelete
	ActionList
	ActionHelp
	ActionVersion
)

const version string = "1.1"
const storeName string = ".git-status"
const commentIndicator string = "#"
const permissions os.FileMode = 0644

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
		case "--ADD":
			fallthrough
		case "-ADD":
			action = ActionAdd

		case "-":
			fallthrough
		case "-D":
			fallthrough
		case "--DEL":
			fallthrough
		case "-DEL":
			fallthrough
		case "-DELETE":
			fallthrough
		case "--DELETE":
			fallthrough
		case "-R":
			fallthrough
		case "--REMOVE":
			fallthrough
		case "-REMOVE":
			action = ActionDelete

		case "-L":
			fallthrough
		case "-LS":
			fallthrough
		case "--LS":
			fallthrough
		case "--LIST":
			fallthrough
		case "-LIST":
			action = ActionList

		case "-A":
			fallthrough
		case "--ALL":
			fallthrough
		case "-ALL":
			showAll = true

		case "-H":
			fallthrough
		case "--HELP":
			fallthrough
		case "-HELP":
			fallthrough
		case "/?":
			action = ActionHelp

		case "-V":
			fallthrough
		case "--VERSION":
			fallthrough
		case "-VERSION":
			action = ActionVersion
		}
	}
	if action == ActionAdd || action == ActionDelete {
		if len(os.Args) >= 3 {
			for _, arg := range os.Args[2:] {
				abs, err := filepath.Abs(arg)
				if err != nil {
					fmt.Println("error parsing path:", err.Error())
					os.Exit(1)
				}
				paths = append(paths, abs)
			}
		} else {
			action = ActionHelp
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
	case ActionAdd:
		registerPaths(paths)
	case ActionDelete:
		removePaths(paths)
	case ActionList:
		listRegistered()
	case ActionHelp:
		printUsage()
	case ActionVersion:
		fmt.Println("git-status v" + version)
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
  -h       Show this help
  -v       Print version`
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
	lines := strings.Split(string(raw), "\n")

	for _, line := range lines {
		path := strings.TrimSpace(line)
		if !contains(registered, path) || path == "" || strings.HasPrefix(path, commentIndicator) {
			registered = append(registered, path)
		}
	}
	// Remove trailing empty lines if they exist
	for len(registered) != 0 && registered[len(registered)-1] == "" {
		registered = registered[:len(registered)-1]
	}
}

func listRegistered() {
	var output string
	var count int
	for _, dir := range registered {
		if len(dir) != 0 && !strings.HasPrefix(dir, commentIndicator) {
			count++
			output += "  " + dir + "\n"
		}
	}
	switch count {
	case 0:
		fmt.Println("No paths registered")
	case 1:
		fmt.Println("1 path registered:")
	default:
		fmt.Println(count, "paths registered:")
	}
	fmt.Println(output)
}

func removePaths(except []string) {
	os.Remove(store)
	f, err := os.OpenFile(store, os.O_RDWR|os.O_CREATE, permissions)
	if err != nil {
		fmt.Println("error saving paths:", err.Error())
		fmt.Println("dumping lines:")
		for _, path := range registered {
			fmt.Println(path)
		}
	}
	defer f.Close()
	firstWrite := true
	for _, path := range registered {
		if !contains(except, path) {
			if !firstWrite {
				f.WriteString("\n")
			}
			f.WriteString(path)
			firstWrite = false
		}
	}
}

func commentPaths(except []string) {
	os.Remove(store)
	f, err := os.OpenFile(store, os.O_RDWR|os.O_CREATE, permissions)
	if err != nil {
		fmt.Println("error saving paths:", err.Error())
		fmt.Println("dumping lines:")
		for _, path := range registered {
			fmt.Println(path)
		}
	}
	defer f.Close()
	for i, path := range registered {
		if !contains(except, path) {
			f.WriteString(path)
		} else {
			f.WriteString(commentIndicator + path)
		}
		if i+1 != len(registered) {
			f.WriteString("\n")
		}
	}
}

func registerPaths(targets []string) {
	f, err := os.OpenFile(store, os.O_RDWR|os.O_APPEND|os.O_CREATE, permissions)
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

		f.WriteString("\n" + target)
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
		if len(path) == 0 || strings.HasPrefix(path, commentIndicator) {
			continue
		}
		if !isRepo(path) {
			fmt.Println(path, "no longer appears to be a git repo, commenting it out")
			commentPaths([]string{path})
			continue
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
			cyan := color.New(color.FgCyan).PrintfFunc()
			yellow := color.New(color.FgYellow).PrintfFunc()
			red := color.New(color.FgRed).PrintfFunc()
			fmt.Printf("%s (", padRight(repo.Name, nameWidth))
			if repo.RemoteBranchError {
				red("%s", padRight("!ERROR!", branchWidth))
			} else {
				fmt.Printf("%s", padRight(repo.RemoteBranch, branchWidth))
			}
			fmt.Printf(") ")
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
	var err error
	status.Name = getRepoName(repo)
	status.RemoteBranch, err = getRemote(repo)
	status.RemoteBranchError = err != nil
	status.Unpulled = getUnpulled(repo, status.RemoteBranch)
	status.Unpushed = getUnpushed(repo, status.RemoteBranch)
	status.Deltas = getDeltas(repo)

	status.ShouldReport = status.Unpulled > 0 || status.Unpushed > 0 || status.Deltas > 0 || status.RemoteBranchError

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

func getRemote(repo string) (string, error) {
	raw, err := getCmdOutput(repo, "git", "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	if err != nil {
		return "", err
	}
	return raw, nil
}

func getUnpulled(repo string, remote string) (unpulled int) {
	raw, err := getCmdOutput(repo, "git", "rev-list", "--count", "HEAD.."+remote)
	if err != nil {
		fmt.Println("error getting unpulled count:", err.Error())
		return -1
	}
	unpulled, err = strconv.Atoi(raw)
	return unpulled
}

func getUnpushed(repo string, remote string) (unpushed int) {
	raw, err := getCmdOutput(repo, "git", "rev-list", "--count", remote+"..HEAD")
	if err != nil {
		fmt.Println("error getting unpushed count:", err.Error())
		return -1
	}
	unpushed, err = strconv.Atoi(raw)
	return unpushed
}

func getDeltas(repo string) int {
	raw, err := getCmdOutput(repo, "git", "status", "--porcelain")
	if err != nil {
		fmt.Println("error getting deltas count:", err.Error())
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
