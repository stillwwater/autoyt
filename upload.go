package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/youtube/v3"
)

// RFC3339 should work but youtube API v3 seems to have an issue
// with time formats that do not include the subsecond part
const ISO8601 = "2006-01-02T15:04:05.000-0700"

type UploadMetadata struct {
	Tags       []string
	Privacy    string
	CategoryId string
}

type UploadCommand struct {
	ClientSecret string
	RootPath     string
	Metadata     UploadMetadata
}

func (self *UploadCommand) Exec(c *Collections) {
	videos := findVideosToUpload(c)
	client := self.getClient(youtube.YoutubeUploadScope)

	service, err := youtube.New(client)
	if err != nil {
		userError("upload: Error creating youtube client.\n%v", err)
	}

	for _, v := range videos {
		self.ytUpload(service, v)
		v.State = Published

		if track, ok := c.Find(v.Audio); ok {
			track.(*Track).State = Published
		}

		if art, ok := c.Find(v.Image); ok {
			art.(*Artwork).State = Published
		}
	}
}

func (self *UploadCommand) ytUpload(service *youtube.Service, video *Video) {
	upload := &youtube.Video{
		Snippet: &youtube.VideoSnippet{
			Title:       video.Title,
			Description: video.Description,
			CategoryId:  self.Metadata.CategoryId,
		},
		Status: &youtube.VideoStatus{PrivacyStatus: self.Metadata.Privacy},
	}

	// API return a 400 Bad Request response if tags is empty
	if len(self.Metadata.Tags) > 0 {
		upload.Snippet.Tags = self.Metadata.Tags
	}

	// Schedule video to be published on a specific time.
	if video.PublishAt != nil {
		// Video must be private to be scheduled
		upload.Status.PrivacyStatus = "private"
		upload.Status.PublishAt = video.PublishAt.Format(ISO8601)
	}

	call := service.Videos.Insert("snippet,status", upload)
	publishVideo(call, video)
}

func publishVideo(call *youtube.VideosInsertCall, video *Video) {
	file, err := os.Open(video.Path)
	defer file.Close()
	if err != nil {
		userError("upload: Unable to open %s\n%v", video.Path, err)
	}

	stop := make(chan bool)
	go userProgress(stop, "upload:", "%s", video)

	res, err := call.Media(file).Do()
	stop <- true
	if err != nil {
		userError("\nupload: Failed to upload %s\n%v", video.Path, err)
	}
	video.UploadId = &res.Id
	userLogRepl("upload:", "%s\n", video)
}

func (self *UploadCommand) getClient(scope string) *http.Client {
	ctx := context.Background()

	b, err := ioutil.ReadFile(self.ClientSecret)
	if err != nil {
		userError("upload: Unable to read client secret.\n%v", err)
	}

	config, err := google.ConfigFromJSON(b, scope)
	if err != nil {
		userError("upload: Unable to parse client secret.\n%v", err)
	}

	config.RedirectURL = "http://localhost:8090"
	tok, err := readToken(self.tokenCacheFile())

	if err != nil {
		authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
		tok, err = newToken(config, authURL)
		if err == nil {
			saveToken(self.tokenCacheFile(), tok)
		}
	}
	return config.Client(ctx, tok)
}

func (self *UploadCommand) tokenCacheFile() string {
	tokenCacheDir := filepath.Join(self.RootPath, ".credentials")
	os.MkdirAll(tokenCacheDir, 0700)
	return filepath.Join(tokenCacheDir, "youtube.json")
}

func exchangeToken(config *oauth2.Config, code string) *oauth2.Token {
	tok, err := config.Exchange(oauth2.NoContext, code)
	if err != nil {
		userError("upload: Unable to retrieve token")
	}
	return tok
}

func startWebServer() (chan string, error) {
	listener, err := net.Listen("tcp", "localhost:8090")
	if err != nil {
		return nil, err
	}
	ch := make(chan string)

	handler := func(w http.ResponseWriter, r *http.Request) {
		code := r.FormValue("code")
		ch <- code
		listener.Close()
		w.Header().Set("Content-Type", "text-plain")

		fmt.Fprintf(w, "Received code: %v\r\n"+
			"You can now safely close this browser window.", code)
	}
	go http.Serve(listener, http.HandlerFunc(handler))
	return ch, nil
}

func newToken(config *oauth2.Config, authURL string) (*oauth2.Token, error) {
	ch, err := startWebServer()
	if err != nil {
		userError("upload: Unable to start a web server")
		return nil, err
	}

	if err = openURL(authURL); err != nil {
		userError("Unable to open auth URL in web server.\n%v", err)
	}
	fmt.Print("Your browser has been opened to an authorization URL.",
		"This program will resume once authorization has been provided\n\n")
	fmt.Println(authURL)

	code := <-ch
	return exchangeToken(config, code), nil
}

// openURL opens a browser window to the specified location.
// From:
//   http://stackoverflow.com/
//      questions/10377243/how-can-i-launch-a-process-that-is-not-a-file-in-go
func openURL(url string) error {
	var err error
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command(
			"rundll32",
			"url.dll,FileProtocolHandler",
			"http://localhost:4001/").Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("Cannot open URL %s on this platform", url)
	}
	return err
}

func readToken(file string) (*oauth2.Token, error) {
	fp, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	tok := &oauth2.Token{}
	err = json.NewDecoder(fp).Decode(tok)
	defer fp.Close()
	return tok, err
}

func saveToken(file string, token *oauth2.Token) {
	fp, err := os.OpenFile(file, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		userError("upload: Unable to cache oauth token.\n%v", err)
	}
	defer fp.Close()
	json.NewEncoder(fp).Encode(token)
}

func findVideosToUpload(c *Collections) []*Video {
	videos := []*Video{}
	for _, v := range c.Schedule {
		if v.State == Scheduled {
			videos = append(videos, v)
		}
	}

	if len(videos) == 0 {
		userError(Err_EmptySchedule)
	}

	sortVideos(videos)
	return videos
}
