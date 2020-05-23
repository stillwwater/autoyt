package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
)

type Template map[string]string

type Editor struct {
	Path       string
	InputArgs  string
	OutputArgs string
	FileFormat string
}

type VideoFormat struct {
	Title          string
	Header         string
	ArtworkCredits string
	TrackCredits   string
	Link           string
	Footer         string
}

type VideoBuilder struct {
	Track     *Track
	Art       *Artwork
	Format    *VideoFormat
	Extension string
}

type templateGen struct {
	c *Collections
	b *strings.Builder
}

// Render video by merging audio from track and artwork image
func (self *Editor) Render(video *Video) error {
	args := strings.Split(self.InputArgs, " ")
	args = append(args, "-i", video.Image, "-i", video.Audio)
	args = append(args, strings.Split(self.OutputArgs, " ")...)
	args = append(args, video.Path)
	cmd := exec.Command(self.Path, args...)

	stop := make(chan bool)
	go userProgress(stop, "render:", "%s", video.Title)

	err := cmd.Start()
	if err != nil {
		return err
	}
	err = cmd.Wait()
	stop <- true
	userLogRepl("render:", "%s  \n", video.Title)
	return err
}

func (self *VideoBuilder) Video(c *Collections, dst string) (*Video, error) {
	title, err := self.Title()
	if err != nil {
		return nil, err
	}

	desc, err := self.Desc(c)
	if err != nil {
		return nil, err
	}

	if dst != "" {
		dst = path.Join(dst, "schedule")
		os.MkdirAll(dst, os.ModePerm)
	}

	filename := title + self.Extension
	dst = path.Join(dst, filename)

	return &Video{
		Title:       title,
		Description: desc,
		Path:        dst,
		State:       Buffered,
		PublishAt:   nil,
		Audio:       self.Track.UniqueId(),
		Image:       self.Art.UniqueId(),
	}, nil
}

func (self *VideoBuilder) Title() (string, error) {
	return buildTemplate(
		self.Format.Title,
		Template{"by": self.Track.By, "title": self.Track.Title},
	)
}

func (self *VideoBuilder) Desc(c *Collections) (string, error) {
	var b strings.Builder
	gen := templateGen{c, &b}

	err := self.writeHeader(gen)
	if err != nil {
		return "", err
	}

	if self.Track.Description != "" {
		b.WriteString(self.Track.Description)
		b.WriteString("\n\n")
	}

	err = self.writeTrackCredits(gen)
	if err != nil {
		return "", err
	}

	err = self.writeArtCredits(gen)
	if err != nil {
		return "", err
	}

	if self.Format.Footer != "" {
		gen.b.WriteByte('\n')
		b.WriteString(self.Format.Footer)
	}
	return b.String(), err
}

func (self *VideoBuilder) writeHeader(gen templateGen) error {
	if self.Format.Header != "" {
		header, err := buildTemplate(
			self.Format.Header,
			Template{"by": self.Track.By, "title": self.Track.Title},
		)
		if err != nil {
			return err
		}
		gen.b.WriteString(header)
		gen.b.WriteString("\n\n")
	}
	return nil
}

func (self *VideoBuilder) writeLinks(gen templateGen, id string) error {
	col, ok := gen.c.Find(strings.ToLower(id))
	if !ok {
		panic(fmt.Sprintf("artist %s not in collections", id))
	}
	artist := col.(*Artist)
	for _, l := range artist.Links {
		link, err := buildTemplate(
			self.Format.Link,
			Template{"link": l},
		)
		if err != nil {
			return err
		}
		gen.b.WriteString(link)
		gen.b.WriteByte('\n')
	}
	return nil
}

func (self *VideoBuilder) writeTrackCredits(gen templateGen) error {
	for _, a := range self.Track.Artists {
		credits, err := buildTemplate(
			self.Format.TrackCredits,
			Template{"artist": a},
		)
		if err != nil {
			return err
		}
		gen.b.WriteString(credits)
		gen.b.WriteByte('\n')

		err = self.writeLinks(gen, a)
		if err != nil {
			return err
		}
		gen.b.WriteByte('\n')
	}
	return nil
}

func (self *VideoBuilder) writeArtCredits(gen templateGen) error {
	credits, err := buildTemplate(
		self.Format.ArtworkCredits,
		Template{"artist": self.Art.Artist},
	)
	if err != nil {
		return err
	}
	gen.b.WriteString(credits)
	gen.b.WriteByte('\n')

	err = self.writeLinks(gen, self.Art.Artist)
	return nil
}

func buildTemplate(format string, template Template) (string, error) {
	var b strings.Builder

	next := func(pos int, b *byte) bool {
		if pos+1 >= len(format) {
			return false
		}
		*b = format[pos+1]
		return true
	}

	for i := 0; i < len(format); i++ {
		var tokB byte
		tokA := format[i]

		if next(i, &tokB) && tokA == '%' && tokB == '(' {
			i += 2
			end := strings.IndexByte(format[i:], ')') + i
			key := format[i:end]

			val, ok := template[key]
			if !ok {
				err := fmt.Sprintf("invalid key '%s' in '%s'", key, format)
				return format, errors.New(err)
			}
			b.WriteString(val)
			i = end
			continue
		}

		b.WriteByte(tokA)
	}
	return b.String(), nil
}
