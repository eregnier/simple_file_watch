package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

type File struct {
	FileSize     int64  `json:"file_size"`
	AbsolutePath string `json:"absolute_path"`
	Found        bool   `json:"found"`
}

type DiffFile struct {
	AbsolutePath string `json:"absolute_path"`
	Operation    string `json:"operation"`
}

var debug = false
var sleepDuration = 1000 * time.Millisecond

func main() {
	callbackCommand, ignoreFolder, ignoreFile, folderToWatch := parseArgs()
	if debug {
		fmt.Println("Command to run on file change : ", callbackCommand)
		fmt.Println("Ignore Folder: ", ignoreFolder)
		fmt.Println("Ignore File: ", ignoreFile)
		fmt.Println("Folder to watch: ", folderToWatch)
	}

	mapIgnoreFolder := make(map[string]bool)
	mapIgnoreFile := make(map[string]bool)
	if ignoreFolder != "" {
		ignoreFolderList := strings.Split(ignoreFolder, ",")
		for _, folder := range ignoreFolderList {
			mapIgnoreFolder[folder] = true
		}
	}
	if ignoreFile != "" {
		ignoreFileList := strings.Split(ignoreFile, ",")
		for _, file := range ignoreFileList {
			mapIgnoreFile[file] = true
		}
	}
	if debug {
		fmt.Println("Ignore Folder List: ", mapIgnoreFolder)
		fmt.Println("Ignore File List: ", mapIgnoreFile)
		fmt.Println()
	}

	previousFileList := getRecursiveFileList(folderToWatch, mapIgnoreFolder, mapIgnoreFile)
	time.Sleep(sleepDuration)
	for {
		fileList := getRecursiveFileList(folderToWatch, mapIgnoreFolder, mapIgnoreFile)

		if debug {
			fmt.Println("\n *** Current Files: ***")
			for file := range fileList {
				fmt.Println("File: ", fileList[file].AbsolutePath, " Size: ", fileList[file].FileSize)
			}
			fmt.Println(" --- \n")
			fmt.Println(" *** Files to watch: ***")
		}
		diff := testDiff(fileList, previousFileList)
		if len(diff) > 0 {
			runCallbackCommand(callbackCommand, diff)
		} else {
			if debug {
				fmt.Println("No change in file list, nothing to do")
			}

		}
		previousFileList = fileList
		time.Sleep(sleepDuration)
	}

}

func testDiff(fileList []File, previousFileList []File) []DiffFile {
	diff := make([]DiffFile, 0)
	previousMap := make(map[string]File)
	newMap := make(map[string]File)
	var allEntries = make(map[string]File)
	for _, file := range fileList {
		allEntries[file.AbsolutePath] = file
		newMap[file.AbsolutePath] = file
	}
	for _, file := range previousFileList {
		allEntries[file.AbsolutePath] = file
		previousMap[file.AbsolutePath] = file
	}
	for filePath := range allEntries {
		var newSize int64 = 0
		var previousSize int64 = 0
		var added = false
		if file, ok := newMap[filePath]; !ok {
			diffFile := DiffFile{AbsolutePath: filePath, Operation: "removed"}
			diff = append(diff, diffFile)
			if debug {
				fmt.Println("File removed: ", file.AbsolutePath)
			}
			added = true
		} else {
			newSize = file.FileSize
		}
		if file, ok := previousMap[filePath]; !ok {
			diffFile := DiffFile{AbsolutePath: filePath, Operation: "added"}
			diff = append(diff, diffFile)
			if debug {
				fmt.Println("File added: ", file.AbsolutePath)
			}
			added = true
		} else {
			previousSize = file.FileSize
		}
		if newSize != previousSize && !added {
			diffFile := DiffFile{AbsolutePath: filePath, Operation: "changed"}
			if debug {
				fmt.Println("File changed: ", allEntries[filePath].AbsolutePath)
			}
			diff = append(diff, diffFile)
		}
	}
	return diff
}

func runCallbackCommand(callbackCommand string, diff []DiffFile) {

	jsonDiffString, err := json.Marshal(diff)
	if err != nil {
		fmt.Println("Error marshalling diff", err)
		return
	}
	subCmd := strings.Split(callbackCommand, " ")

	simpleDiffEnv := fmt.Sprintf("SIMPLE_FILE_WATCH_CHANGES='%s'", jsonDiffString)
	fmt.Println("Running command: ", simpleDiffEnv)
	cmd := exec.Command(subCmd[0], subCmd[1:]...)
	cmd.Env = append(cmd.Environ(), simpleDiffEnv)
	out, err := cmd.Output()
	if err != nil {
		fmt.Println("")
		fmt.Println(err)
		fmt.Println("Error running callback command")
	}
	fmt.Println(string(out))
}

func parseArgs() (string, string, string, string) {

	cmd := ""
	ignoreFolder := ""
	ignoreFile := ""
	folderToWatch := ""
	for i, arg := range os.Args {
		if arg == "-x" || arg == "--run-command" {
			if i+1 <= len(os.Args)-1 {
				cmd = os.Args[i+1]
			}
		}
		if arg == "--ignore-folder" || arg == "-ifo" {
			if i+1 <= len(os.Args)-1 {
				ignoreFolder = os.Args[i+1]
			}

		}
		if arg == "--ignore-file" || arg == "-ifi" {
			if i+1 <= len(os.Args)-1 {
				ignoreFile = os.Args[i+1]
			}
		}
		if arg == "--watch-folder" || arg == "-w" {
			if i+1 <= len(os.Args)-1 {
				folderToWatch = os.Args[i+1]
			}
		}
		if arg == "--debug" || arg == "-d" {
			debug = true
		}
		if arg == "--help" || arg == "-h" {
			fmt.Println("Usage: ")
			fmt.Println("  --help, -h : Show help")
			fmt.Println("  --run-command, -x : Command to run on file change")
			fmt.Println("  --ignore-folder, -ifo : Comma separated list of folders to ignore")
			fmt.Println("  --ignore-file, -ifi : Comma separated list of files to ignore")
			fmt.Println("  --watch-folder, -w : Folder to watch")
			fmt.Println("  --sleep, -s : Sleep duration between checks")
			fmt.Println("  --debug, -d : Enable debug mode")
			os.Exit(0)
		}

		if arg == "--sleep" || arg == "-s" {
			if i+1 <= len(os.Args)-1 {
				duration, err := time.ParseDuration(os.Args[i+1] + "ms")
				if err != nil {
					fmt.Println("Invalid duration: ", os.Args[i+1])
					os.Exit(1)
				}
				sleepDuration = duration
			}
		}
	}
	if cmd == "" {
		fmt.Println("Callback command using -x is required")
		os.Exit(1)

	}
	if folderToWatch == "" {
		fmt.Println("Folder to watch using --watch-folder is required")
		os.Exit(1)
	}
	return cmd, ignoreFolder, ignoreFile, folderToWatch

}

func getRecursiveFileList(path string, ignoreFolder map[string]bool, ignoreFile map[string]bool) []File {
	files, err := os.ReadDir(path)
	if err != nil {
		fmt.Println(err)
	}
	var fileList []File
	for _, file := range files {
		if file.IsDir() {
			subFolder := path + "/" + file.Name()
			if !ignoreFolder[subFolder] {
				fileList = append(fileList, getRecursiveFileList(subFolder, ignoreFolder, ignoreFile)...)
			} else {
				if debug {
					fmt.Println("Ignoring folder: ", file.Name())
				}
			}
		} else {
			if !ignoreFile[file.Name()] {
				fileInfo, err := file.Info()
				if err != nil {
					fmt.Println(err)
					continue
				}
				entry := File{AbsolutePath: path + "/" + file.Name(), FileSize: fileInfo.Size(), Found: false}
				fileList = append(fileList, entry)
			} else {
				if debug {
					fmt.Println("Ignoring file: ", file.Name())
				}
			}

		}
	}
	return fileList
}
