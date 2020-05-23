package main

import (
	"errors"
	"fmt"
	"math"
	"os"
	"time"
)

const (
	Err_NoBufferedTrack   = "No new music to schedule."
	Err_NoBufferedArtwork = "No new artwork to schedule."
	Err_EmptySchedule     = "Empty schedule."
	Err_PublishedVideo    = "Cannot unschedule published video."
	Err_InvalidUploadTime = "Upload time %s is not valid (expected hh:ss:mm)."
)

type Schedule struct {
	Tracks  []*Track
	Artwork []*Artwork
	Count   int
}

type ScheduleOptions struct {
	Short bool `opt:"-s"`
}

type ScheduleCommand struct {
	DataDir         string
	Function        string
	Editor          Editor
	Format          VideoFormat
	UploadFrequency int
	UploadTimeUTC   string
	Options         ScheduleOptions
}

// Try to schedule a videos by finding a suitable track and artwork
func NewSchedule(c *Collections) (Schedule, error) {
	tracks := []*Track{}
	artwork := []*Artwork{}

	for i := len(c.Tracks) - 1; i >= 0; i-- {
		t := c.Tracks[i]
		if t.State == Buffered {
			tracks = append(tracks, t)
		}
	}
	if len(tracks) == 0 {
		return Schedule{}, errors.New(Err_NoBufferedTrack)
	}

	for i := len(c.Artwork) - 1; i >= 0; i-- {
		a := c.Artwork[i]
		if a.State == Buffered {
			artwork = append(artwork, a)
		}
	}
	if len(artwork) == 0 {
		return Schedule{}, errors.New(Err_NoBufferedArtwork)
	}

	count := int(math.Min(float64(len(tracks)), float64(len(artwork))))
	return Schedule{tracks[:count], artwork[:count], count}, nil
}

func (self *ScheduleCommand) Exec(c *Collections) {
	switch self.Function {
	case "undo":
		if len(c.Schedule) == 0 {
			userError(Err_EmptySchedule)
		}

		vid := c.Schedule[len(c.Schedule)-1]
		if vid.State == Published {
			userError(Err_PublishedVideo)
		}

		// Unschedule art and track associated with the video
		if track, ok := c.Find(vid.Audio); ok {
			track.(*Track).State = Buffered
		}
		if art, ok := c.Find(vid.Image); ok {
			art.(*Artwork).State = Buffered
		}

		// Remove rendered video
		c.Schedule = c.Schedule[:len(c.Schedule)-1]
		os.Remove(vid.Path)
		userLog("undo:", vid.Title)

	case "list":
		if len(c.Schedule) == 0 {
			userError(Err_EmptySchedule)
		}
		if self.Options.Short {
			printSchedule(c.Schedule, 0)
			break
		}
		// Print shorter versions of previous videos in the schedule
		// and a full description on the newest scheduled video.
		vid := c.Schedule[len(c.Schedule)-1]
		printSchedule(c.Schedule, 1)
		fmt.Println()
		describeVideo(vid)

	default:
		// Create schedule by rendering all buffered items in schedule
		self.renderAll(c)
	}
}

func (self *ScheduleCommand) renderAll(c *Collections) int {
	schedule, err := NewSchedule(c)
	if err != nil {
		userError(err.Error())
	}

	startTime, ok := latestScheduledTime(c)
	now := time.Now()
	if !ok {
		startTime = now
	}

	// Schedule has items in reverse order such that the most recent
	// tracks are in the beggining, we want to upload videos in
	// chronological order.
	for i := schedule.Count - 1; i >= 0; i-- {
		track := schedule.Tracks[i]
		art := schedule.Artwork[i]

		build := VideoBuilder{track, art, &self.Format, self.Editor.FileFormat}
		vid, err := build.Video(c, self.DataDir)
		if err != nil {
			userError(err.Error())
		}

		timeSlot := self.scheduleTime(startTime, schedule.Count-i)
		if timeSlot.After(now) {
			vid.PublishAt = &timeSlot
		} else {
			vid.PublishAt = &now
		}

		if err = self.Editor.Render(vid); err != nil {
			userError(err.Error())
		}

		track.State = Scheduled
		art.State = Scheduled
		vid.State = Scheduled
		c.Schedule = append(c.Schedule, vid)
	}
	return schedule.Count
}

func (self *ScheduleCommand) scheduleTime(start time.Time, pos int) time.Time {
	uploadTime, err := time.Parse("15:04:05", self.UploadTimeUTC)
	if err != nil {
		userError(Err_InvalidUploadTime, self.UploadTimeUTC)
	}

	// If time.Now() happens to have a time after start
	// the video will be scheduled to be uploaded immediately
	start = start.AddDate(0, 0, pos*self.UploadFrequency)
	return mergeDateTimeUTC(start, uploadTime)
}

func latestScheduledTime(c *Collections) (time.Time, bool) {
	if len(c.Schedule) == 0 {
		return time.Time{}, false
	}

	// Avoid searching through entire collection to determine a time
	count := 256
	if len(c.Schedule) < count {
		count = len(c.Schedule)
	}

	latest := time.Now()
	if c.Schedule[0].PublishAt != nil {
		latest = *c.Schedule[0].PublishAt
	}

	// Most recent videos are at the end of the collection
	for i := count - 1; i >= 1; i-- {
		video := c.Schedule[i]
		if video.PublishAt.After(latest) {
			latest = *video.PublishAt
		}
	}

	return latest, true
}

func mergeDateTimeUTC(date, t time.Time) time.Time {
	return time.Date(
		date.Year(),
		date.Month(),
		date.Day(),
		t.Hour(),
		t.Minute(),
		t.Second(),
		0,
		time.UTC)
}

func printSchedule(schedule []*Video, start int) {
	now := time.Now()
	videos := schedule[:len(schedule)-start]
	sortVideos(videos)
	i := 0
	for _, v := range videos {
		t := v.PublishAt
		// List videos that have already been uploaded but will only
		// be published at a later time.
		if v.State != Scheduled && (t == nil || t.Before(now)) {
			continue
		}
		i += 1
		fmt.Printf("%d. %s\n", i, v)
	}
}
