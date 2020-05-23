package main

import (
	"fmt"
	"math"
	"runtime"
	"strings"
)

type DescOptions struct {
	N     int    `opt:"-n"`
	Count int    `opt:"-c"`
	All   bool   `opt:"-a"`
	Link  string `opt:"-l"`
}

type DescCommand struct {
	Args      []string
	Format    VideoFormat
	Extension string
	Options   DescOptions
}

func (self *DescCommand) Exec(c *Collections) {
	if len(self.Args) > 0 && self.Options.Link != "" {
		// Args are links for an artist
		name := self.Options.Link
		UpdateArtistLinks(c, name, self.Args)
		return
	}

	schedule, err := NewSchedule(c)
	if err != nil {
		userError(err.Error())
	}

	clampOpt(1, self.Options.N, &self.Options.N)
	clampOpt(self.Options.N, schedule.Count, &self.Options.Count)

	if self.Options.N > schedule.Count {
		// Nothing scheduled on index N
		return
	}

	if len(self.Args) > 0 {
		// Add description to a track
		// Each argument is a new line in the description
		desc := strings.Join(self.Args, "\n")
		schedule.Tracks[self.Options.N-1].Description = desc
	}

	if self.Options.All {
		self.Options.Count = schedule.Count
	}
	for i := self.Options.N - 1; i < self.Options.Count; i++ {
		track := schedule.Tracks[i]
		art := schedule.Artwork[i]

		// Build video but don't render it, we just want a preview of
		// what the final video information will look like.
		build := VideoBuilder{track, art, &self.Format, self.Extension}
		vid, err := build.Video(c, "")

		if err != nil {
			userError(err.Error())
		}

		fmt.Println()
		describeVideo(vid)
		fmt.Println()
	}
}

// Insert or update links for artist matching a name. If the artist
// does not exist a new Artist will be appended to collections.
func UpdateArtistLinks(c *Collections, name string, links []string) {
	col, ok := c.Find(strings.ToLower(name))
	if ok {
		artist := col.(*Artist)
		appendUnique(&artist.Links, links...)
		return
	}
	artist := Artist{name, []string{}}
	appendUnique(&artist.Links, links...)
	c.Artists = append(c.Artists, &artist)
}

func describeVideo(vid *Video) {
	t := vid.String()
	line := strings.Repeat("-", len(t))
	if runtime.GOOS == "windows" {
		fmt.Printf("%s\n%s\n%s\n", t, line, vid.Description)
		return
	}
	fmt.Printf("\033[0;36m%s\033[0m\n%s\n%s\n", t, line, vid.Description)
}

func clampOpt(min, max int, value *int) {
	maxf := math.Min(float64(*value), float64(max))
	*value = int(math.Max(maxf, float64(min)))
}

func appendUnique(a *[]string, str ...string) {
	for _, item := range str {
		contains := false
		for _, s := range *a {
			if s == item {
				contains = true
				break
			}
		}
		if contains {
			continue
		}
		*a = append(*a, item)
	}
}
