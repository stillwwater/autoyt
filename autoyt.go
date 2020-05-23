package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	Err_ExpectedArgs     = "Expected at least %d arguments for %s."
	Err_ConfigParse      = "Failed to parse config file: %s.\n%v"
	Err_MissingOption    = "Expected value for option '%s'."
	Err_IncorrectOptType = "Expected %s value from option '%s'."
	DefaultConfigPath    = "~/.autoyt/config.json"
)

const Usage = `
Usage: autoyt [command] [options]

commands:
    add f [path]          Add music or art to buffer
        f                 Can be either music, art or undo
        path              Path to the music or art file, for
                          artwork this path can be a url.
        -a <artists>      Set the names for the artists for music or art
                          (comma separated). For artists this value is
                          inferred from the file name but can be overridden
                          with this option.
        -by <artist>      Override the 'by' part of the track title.
        -n <name>         Override the 'name' part of the track title.
        -d <description>  Add a description to the music. This value will
                          be appended to the video description.
        -mv               Move file from path instead of copying it.

    desc [items...]       Preview or make changes to video descriptions
                          before they are scheduled or published.
                          If no arguments are passed, show a description of
                          the latest buffered video.
        items             Items to append to the video description, each
                          item is a new line in the description.
        -n <N>            Describe a specific video (default=1).
        -c <N>            Number of videos to display (default=1).
        -a                Describe all videos in buffer.
        -l <artist>       Items will be added to a specific artist, this
                          option can be used to add links related to an
                          artist which will be shown in the credits section
                          of the video description.

    schedule [f]          Render and schedule videos in a buffer.
        f                 Can be one of undo or list. Undo deletes the
                          scheduled video. List shows scheduled videos.
        -s                Print shorter version of list.

    upload                Upload all scheduled videos to YouTube.
    status                Print number of scheduled and published videos.
    json                  Print stored data as json.
`

type Config struct {
	RootPath        string
	DataPath        string
	CollectionsPath string
	Ffmpeg          Editor
	VideoFormat     VideoFormat
	ClientSecret    string
	Metadata        UploadMetadata
	UploadFrequency int
	UploadTimeUTC   string
}

var configPaths = []string{
	"config.json",
	expandHomePath("~/.config/autoyt/config.json"),
	expandHomePath(DefaultConfigPath),
}

var defaultConfig = Config{
	RootPath:        "~/.autoyt",
	DataPath:        "~/.autoyt/data",
	CollectionsPath: "~/.autoyt/collections.json",
	ClientSecret:    "~/.autoyt/client_secret.json",
	Ffmpeg: Editor{
		Path:       "ffmpeg",
		InputArgs:  "-r 1 -loop 1",
		OutputArgs: "-acodec copy -r 1 -shortest",
		FileFormat: ".mp4",
	},
	VideoFormat: VideoFormat{
		Title:          "%(by) - %(title)",
		Header:         "%(by) - %(title)",
		TrackCredits:   "%(artist)",
		ArtworkCredits: "Artwork by %(artist)",
		Link:           "- %(link)",
		Footer:         "",
	},
	Metadata: UploadMetadata{
		Tags:       []string{},
		Privacy:    "public",
		CategoryId: "10",
	},
	UploadFrequency: 1,
	UploadTimeUTC:   "12:00:00",
}

func main() {
	args := os.Args[1:]
	help := Usage[1 : len(Usage)-1]
	if len(args) == 0 {
		fmt.Println(help)
		os.Exit(1)
	}

	config := readConfig()
	collections := readCollections(expandHomePath(config.CollectionsPath))

	switch args[0] {
	case "add":
		opt := parseOptions(&args, AddOptions{}).(AddOptions)
		dlopt := parseOptions(&args, DownloadOptions{}).(DownloadOptions)
		expectArgs(args, "add", 3)

		download := DownloadCommand{
			DataDir: expandHomePath(config.DataPath),
			Options: dlopt,
		}

		add := AddCommand{
			CollectionName: args[1],
			SrcPath:        args[2],
			DataDir:        expandHomePath(config.DataPath),
			Download:       download,
			Options:        opt,
		}
		add.Exec(&collections)

	case "desc":
		opt := parseOptions(&args, DescOptions{}).(DescOptions)
		expectArgs(args, "desc", 1)

		desc := DescCommand{
			Args:      args[1:],
			Format:    config.VideoFormat,
			Extension: config.Ffmpeg.FileFormat,
			Options:   opt,
		}
		desc.Exec(&collections)

	case "schedule":
		expectArgs(args, "schedule", 1)
		opt := parseOptions(&args, ScheduleOptions{}).(ScheduleOptions)

		var fn string
		if len(args) > 1 {
			fn = args[1]
		}

		schedule := ScheduleCommand{
			DataDir:         expandHomePath(config.DataPath),
			Function:        fn,
			Editor:          config.Ffmpeg,
			Format:          config.VideoFormat,
			UploadFrequency: config.UploadFrequency,
			UploadTimeUTC:   config.UploadTimeUTC,
			Options:         opt,
		}
		schedule.Exec(&collections)

	case "upload":
		expectArgs(args, "upload", 1)

		upload := UploadCommand{
			ClientSecret: expandHomePath(config.ClientSecret),
			RootPath:     expandHomePath(config.RootPath),
			Metadata:     config.Metadata,
		}
		upload.Exec(&collections)

	case "status":
		fmt.Println(collections.videoStatus())
		return

	case "json":
		j, _ := json.Marshal(&collections)
		fmt.Println(string(j))
		return

	case "--help":
		fmt.Println(help)
		return

	default:
		fmt.Println(help)
		os.Exit(1)
	}
	os.MkdirAll(expandHomePath(config.RootPath), os.ModePerm)
	writeCollections(expandHomePath(config.CollectionsPath), &collections)
}

func readCollections(path string) Collections {
	result := Collections{
		[]*Track{},
		[]*Artwork{},
		[]*Video{},
		[]*Artist{},
		map[string]Collection{},
	}
	result.UpdateIndexes()

	if !fileExists(path) {
		// Return empty collections
		return result
	}
	file, err := ioutil.ReadFile(path)

	if err != nil {
		panic(err)
	}
	json.Unmarshal(file, &result)
	return result
}

func writeCollections(path string, collections *Collections) {
	file, err := json.MarshalIndent(collections, "", "    ")

	if err != nil {
		panic(err)
	}
	ioutil.WriteFile(path, file, os.ModePerm)
}

func readConfig() *Config {
	var path string

	for _, p := range configPaths {
		if fileExists(p) {
			path = p
			break
		}
	}
	file, err := ioutil.ReadFile(path)

	if err != nil {
		os.MkdirAll(expandHomePath(defaultConfig.RootPath), os.ModePerm)
		writeConfig(expandHomePath(DefaultConfigPath), &defaultConfig)
		return &defaultConfig
	}

	config := new(Config)
	if err := json.Unmarshal(file, config); err != nil {
		userError(Err_ConfigParse, path, err)
	}
	return config
}

func writeConfig(path string, config *Config) {
	file, err := json.MarshalIndent(config, "", "    ")

	if err != nil {
		panic(err)
	}
	ioutil.WriteFile(path, file, os.ModePerm)
}

func parseOptions(args *[]string, val interface{}) interface{} {
	positional := make([]string, 0, len(*args))
	tags := make(map[string]string)
	t := reflect.TypeOf(val)
	s := reflect.New(reflect.TypeOf(val)).Elem()

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tag := f.Tag.Get("opt")
		tags[tag] = f.Name
	}

	for i := 0; i < len(*args); i++ {
		a := (*args)[i]
		fieldName, ok := tags[a]

		if !ok {
			positional = append(positional, a)
			continue
		}

		f := s.FieldByName(fieldName)
		if !f.IsValid() || !f.CanSet() {
			positional = append(positional, a)
			continue
		}

		switch f.Kind() {
		case reflect.String:
			if i+1 >= len(*args) {
				userError(Err_MissingOption, a)
			}
			f.SetString((*args)[i+1])
			i += 1
		case reflect.Bool:
			f.SetBool(true)
		case reflect.Int:
			if i+1 >= len(*args) {
				userError(Err_MissingOption, a)
			}
			n, err := strconv.Atoi((*args)[i+1])
			if err != nil {
				userError(Err_IncorrectOptType, "integer", a)
			}
			f.SetInt(int64(n))
			i += 1
		default:
			positional = append(positional, a)
		}
	}
	*args = positional
	return s.Interface()
}

func expectArgs(args []string, name string, length int) {
	if len(args) >= length {
		return
	}
	userError(Err_ExpectedArgs, length-1, name)
}

func expandHomePath(path string) string {
	usr, err := user.Current()
	if err != nil {
		userError(err.Error())
	}
	if path == "~" {
		return usr.HomeDir
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(usr.HomeDir, path[2:])
	}
	return path
}

func userProgress(stop chan bool, mod, format string, args ...interface{}) {
	progress := [...]byte{'|', '/', '-', '|', '-', '\\'}
	msg := fmt.Sprintf(format, args...)
	for i := 0; ; i++ {
		select {
		case <-stop:
			return
		default:
			time.Sleep(60 * time.Millisecond)
			tok := progress[i%len(progress)]
			userLogRepl(mod, "%s %c ", msg, tok)
		}
	}
}

func userError(format string, args ...interface{}) {
	var pre string
	if runtime.GOOS == "windows" {
		pre = "error:"
	} else {
		pre = "\033[0;31;1merror:\033[0m"
	}
	err := fmt.Sprintf(format, args...)
	fmt.Printf("%s %s\n", pre, err)
	os.Exit(1)
}

func userLog(mod, format string, args ...interface{}) {
	if runtime.GOOS != "windows" {
		mod = fmt.Sprintf("\033[0;36;1m%s\033[0m", mod)
	}
	log := fmt.Sprintf(format, args...)
	fmt.Printf("%s %s\n", mod, log)
}

func userLogRepl(mod, format string, args ...interface{}) {
	if runtime.GOOS != "windows" {
		mod = fmt.Sprintf("\033[0;36;1m%s\033[0m", mod)
	}
	log := fmt.Sprintf(format, args...)
	fmt.Printf("\r%s %s", mod, log)
}

// Copy file contents from src path to dst path
func fileCopy(src, dst string) (int, error) {
	buf, err := ioutil.ReadFile(src)
	if err != nil {
		return 0, err
	}

	err = ioutil.WriteFile(dst, buf, 0644)
	if err != nil {
		return 0, err
	}
	return len(buf), nil
}

// Check that a path exists
func fileExists(path string) bool {
	if _, err := os.Stat(path); err != nil {
		return !os.IsNotExist(err)
	}
	return true
}

// Check if path exists and is a directory
func isDirectory(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}
