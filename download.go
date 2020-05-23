package main

import (
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
)

const (
	Err_DownloadFailed   = "Failed to download '%s' (%s)."
	Err_UnknownExtension = "Cannot determine file extension, use -ext flag"
)

type DownloadOptions struct {
	FileExtension string `opt:"-ext"`
}

type DownloadCommand struct {
	DataDir string
	Options DownloadOptions
}

func (self *DownloadCommand) GetArtwork(urlPath string) string {
	dst := path.Join(self.DataDir, ".cache")
	os.MkdirAll(dst, os.ModePerm)

	stop := make(chan bool)
	go userProgress(stop, "download:", urlPath)

	res, err := http.Get(urlPath)
	stop <- true
	userLogRepl("download:", "%s  \n", urlPath)

	if err != nil {
		userError(Err_DownloadFailed, urlPath, err)
	}
	defer res.Body.Close()

	urlParts := strings.Split(urlPath, "/")
	dst = path.Join(dst, urlParts[len(urlParts)-1])

	if self.Options.FileExtension != "" {
		dst += self.Options.FileExtension
	} else if !validFileName(dst) {
		userError(Err_UnknownExtension)
	}

	file, err := os.Create(dst)
	if err != nil {
		userError(Err_DownloadFailed, urlPath, err)
	}
	defer file.Close()

	_, err = io.Copy(file, res.Body)
	if err != nil {
		userError(Err_DownloadFailed, urlPath, err)
	}
	return dst
}

func validFileName(name string) bool {
	exts := [...]string{".png", ".jpg", ".jpeg", ".gif", ".bmp"}
	for _, ext := range exts {
		if strings.HasSuffix(name, ext) {
			return true
		}
	}
	return false
}

func isUrl(name string) bool {
	u, err := url.Parse(name)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return false
	}
	return true
}
