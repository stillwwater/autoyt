package main

import (
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const (
	Err_ImmutableResource = "Cannot update %s as it is already schedule or published."
	Err_CreateResource    = "Could not create %s."
	Err_FileNotFound      = "File or directory '%s' does not exist."
)

type AddOptions struct {
	Artist   string `opt:"-a"`
	By       string `opt:"-by"`
	Name     string `opt:"-n"`
	Desc     string `opt:"-d"`
	MoveFile bool   `opt:"-mv"`
}

type AddCommand struct {
	CollectionName string
	SrcPath        string
	DataDir        string
	Download       DownloadCommand
	Options        AddOptions
}

func (self *AddCommand) Exec(c *Collections) {
	paths := listFilePaths(self.SrcPath)
	dst := path.Join(self.DataDir, self.CollectionName)
	os.MkdirAll(dst, os.ModePerm)

	switch self.CollectionName {
	case "music":
		if len(paths) > 0 && paths[0] == "undo" {
			track := c.Tracks[len(c.Tracks)-1]
			if track.State != Buffered {
				userError(Err_ImmutableResource, "music")
			}
			c.Tracks = c.Tracks[:len(c.Tracks)-1]

			// Remove track from disk
			os.Remove(track.Path)
			userLog("undo:", track.Path)
			return
		}
		for _, p := range paths {
			self.execAddMusic(c, p, dst)
		}
	case "art":
		if len(paths) > 0 && paths[0] == "undo" {
			art := c.Artwork[len(c.Artwork)-1]
			if art.State != Buffered {
				userError(Err_ImmutableResource, "art")
			}
			c.Artwork = c.Artwork[:len(c.Artwork)-1]

			// Remove artwork from disk
			os.Remove(art.Path)
			userLog("undo:", art.Path)
			return
		}
		for _, p := range paths {
			self.execAddArtwork(c, p, dst)
		}
	}
}

func (self *AddCommand) execAddMusic(c *Collections, src, dst string) {
	track, err := NewTrack(src, dst, self.Options)

	if err != nil {
		userError(Err_CreateResource, "music")
	}
	AddTrack(c, *track)
}

func (self *AddCommand) execAddArtwork(c *Collections, src, dst string) {
	if isUrl(src) {
		src = self.Download.GetArtwork(src)
		self.Options.MoveFile = true
	}
	art, err := NewArtwork(src, dst, self.Options)

	if err != nil {
		userError(Err_CreateResource, "artwork")
	}
	AddArtwork(c, *art)
}

func listFilePaths(src string) []string {
	if isDirectory(src) {
		paths := make([]string, 0)
		files, err := ioutil.ReadDir(src)

		if err != nil {
			userError(Err_FileNotFound, src)
		}

		for _, f := range files {
			filePath := path.Join(src, f.Name())
			paths = append(paths, filePath)
		}
		return paths
	}
	return []string{src}
}

// Create artwork by copying or moving src file to a file in dst
// directory with the same name.
func NewArtwork(src, dst string, opt AddOptions) (*Artwork, error) {
	file := filepath.Base(src)
	dst = path.Join(dst, file)

	if src != dst {
		var err error
		if opt.MoveFile {
			os.Rename(src, dst)
		} else {
			_, err = fileCopy(src, dst)
		}
		if err != nil {
			return nil, err
		}
	}
	return &Artwork{opt.Artist, dst, Buffered}, nil
}

// Add artwork to collection. If an artwork with the same UniqueId already
// exists the previous artwork will be replaced as long as the previous
// artwork has not already been scheduled.
//
// This function may update the Artists collection to ensure
// artwork.Artist exists in the collection.
func AddArtwork(c *Collections, artwork Artwork) {
	updateArtists(c, artwork.Artist)
	for i, a := range c.Artwork {
		if a.UniqueId() == artwork.UniqueId() {
			if a.State != artwork.State && a.State != Removed {
				userError(Err_ImmutableResource, "art")
				return
			}
			c.Artwork[i] = &artwork
			return
		}
	}
	c.Artwork = append(c.Artwork, &artwork)
}

// Create track by copying or moving src file to a file in dst
// directory with the same name.
func NewTrack(src, dst string, opt AddOptions) (*Track, error) {
	file := filepath.Base(src)
	// Try to infer track name and artist
	title, artist := trackInfo(file)

	if opt.Name != "" {
		title = opt.Name
	}

	if opt.By != "" {
		artist = opt.By
	}

	dst = path.Join(dst, file)
	var err error

	if opt.MoveFile {
		err = os.Rename(src, dst)
	} else {
		_, err = fileCopy(src, dst)
	}
	if err != nil {
		return nil, err
	}

	artists := inferArtists(title, artist, opt)

	// By default no description is added
	return &Track{title, artist, artists, opt.Desc, dst, Buffered}, nil
}

// Add track to collection. If an artwork with the same UniqueId already
// exists the previous artwork will be replaced as long as the previous
// artwork has not already been scheduled.
//
// This function may update the Artists collection to ensure
// track.Artist exists in the collection.
func AddTrack(c *Collections, track Track) {
	updateArtists(c, track.Artists...)
	for i, t := range c.Tracks {
		if t.UniqueId() == track.UniqueId() {
			if t.State != track.State && t.State != Removed {
				userError(Err_ImmutableResource, "music")
				return
			}
			c.Tracks[i] = &track
			return
		}
	}
	c.Tracks = append(c.Tracks, &track)
}

func inferArtists(title, artist string, opt AddOptions) (artists []string) {
	if opt.Artist != "" {
		artists = strings.Split(opt.Artist, ",")
	} else {
		// Try to infer multiple artist names
		artists = splitStrings(artist, []string{"&", "x", "X", "+"})
		features := splitStrings(title, []string{"feat.", "Feat.", "ft."})
		if len(features) > 1 {
			artists = append(artists, features[1:]...)
		}
	}

	for i, a := range artists {
		artists[i] = strings.TrimSpace(a)
	}
	return
}

func trackInfo(path string) (title, artist string) {
	// Remove file extension
	path = strings.TrimSuffix(path, filepath.Ext(path))
	info := strings.Split(path, "-")

	if len(info) >= 1 {
		artist = strings.TrimSpace(info[0])
	}
	if len(info) >= 2 {
		title = strings.TrimSpace(info[1])
	}
	return
}

func updateArtists(c *Collections, artists ...string) {
	for _, artist := range artists {
		_, ok := c.Find(strings.ToLower(artist))
		if ok {
			return
		}
		c.Artists = append(c.Artists, &Artist{artist, []string{}})
	}
}

func splitStrings(s string, sep []string) []string {
	var result []string
	tokens := strings.Split(s, " ")
	start := 0

	for i, tok := range tokens {
		for _, sp := range sep {
			if tok == sp {
				part := strings.Join(tokens[start:i], " ")
				if part == "" {
					continue
				}
				result = append(result, part)
				start = i + 1
			}
		}
	}
	if part := strings.Join(tokens[start:], " "); part != "" {
		result = append(result, part)
	}
	return result
}
