package main

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	Buffered = iota
	Scheduled
	Published
	Removed
)

type ItemState int

type Collection interface {
	UniqueId() string
}

type Video struct {
	Title       string
	Description string
	Path        string
	State       ItemState
	PublishAt   *time.Time
	UploadId    *string
	Audio       string
	Image       string
}

type Track struct {
	Title       string
	By          string
	Artists     []string
	Description string
	Path        string
	State       ItemState
}

type Artwork struct {
	Artist string
	Path   string
	State  ItemState
}

type Artist struct {
	Name  string
	Links []string
}

type Collections struct {
	Tracks   []*Track
	Artwork  []*Artwork
	Schedule []*Video
	Artists  []*Artist
	Indexes  map[string]Collection `json:"-"`
}

func (self *Collections) UpdateIndexes() {
	for _, t := range self.Tracks {
		self.Indexes[t.UniqueId()] = t
	}
	for _, a := range self.Artwork {
		self.Indexes[a.UniqueId()] = a
	}
	for _, v := range self.Schedule {
		self.Indexes[v.UniqueId()] = v
	}
	for _, a := range self.Artists {
		self.Indexes[a.UniqueId()] = a
	}
}

func (self *Collections) Find(id string) (Collection, bool) {
	sum := len(self.Artists) +
		len(self.Artwork) +
		len(self.Tracks) +
		len(self.Schedule)

	if sum != len(self.Indexes) {
		self.UpdateIndexes()
	}

	c, ok := self.Indexes[id]
	return c, ok
}

func sortVideos(videos []*Video) {
	sort.Slice(videos, func(i, j int) bool {
		// A nil time value means the video will be published
		// immediately.
		if videos[i].PublishAt == nil {
			return true
		}
		if videos[j].PublishAt == nil {
			return false
		}
		return videos[i].PublishAt.Before(*videos[j].PublishAt)
	})
}

func (self *Collections) videoStatus() string {
	var sum [4]int
	for _, v := range self.Schedule {
		if int(v.State) > len(sum) {
			panic(v)
		}
		sum[v.State]++
	}
	s := sum[Scheduled]
	p := sum[Published]
	return fmt.Sprintf("scheduled: %d, published: %d", s, p)
}

func (self *Track) UniqueId() string {
	return self.Path
}

func (self *Artwork) UniqueId() string {
	return self.Path
}

func (self *Artist) UniqueId() string {
	return strings.ToLower(self.Name)
}

func (self *Video) UniqueId() string {
	return self.Path
}

func (self *Video) String() string {
	if self.PublishAt == nil {
		return self.Title
	}
	timeStamp := self.PublishAt.Format("2006-01-02 15:04")
	str := fmt.Sprintf("%s @(%s)", self.Title, timeStamp)
	if self.UploadId == nil {
		return str
	}
	return fmt.Sprintf("%s (video id: %s)", str, *self.UploadId)
}
