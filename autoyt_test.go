package main

import (
	"strconv"
	"strings"
	"testing"
)

func TestParseOptions(t *testing.T) {
	type Options struct {
		OptionStr  string `opt:"-s"`
		OptionInt  int    `opt:"-i"`
		OptionBool bool   `opt:"-b"`
	}
	tests := []struct {
		in         string
		expectArgs string
		expectOpt  Options
	}{
		{"", "", Options{}},
		{"aaa aaa", "aaa aaa", Options{}},
		{"-s b -i 12 -b a", "a", Options{"b", 12, true}},
		{"-s bb 12 -i 12 6", "12 6", Options{"bb", 12, false}},
	}

	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			args := strings.Split(tt.in, " ")
			opt := parseOptions(&args, Options{}).(Options)
			res := strings.Join(args, " ")

			if tt.expectArgs != res {
				t.Errorf("expected args %s, got %s", args, res)
			}
			if opt != tt.expectOpt {
				t.Errorf("expected opt %v, got %v", tt.expectOpt, opt)
			}
		})
	}
}

func TestBuildTemplate(t *testing.T) {
	tests := []struct {
		format   string
		template Template
		expect   string
	}{
		{"%(abc) - %(d)", Template{"abc": "ABC", "d": "D"}, "ABC - D"},
		{"%%(abc)%%(d)%", Template{"abc": "ABC", "d": "D"}, "%ABC%D%"},
		{"%(ab)c)", Template{"ab": "ABC"}, "ABCc)"},
		{"%()", Template{"": "A"}, "A"},
	}

	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			s, err := buildTemplate(tt.format, tt.template)
			if err != nil {
				t.Error(err)
			}
			if s != tt.expect {
				t.Errorf("expected %s, got %s\n", tt.expect, s)
			}
		})
	}
}

func TestVideoBuilder(t *testing.T) {
	const title = "TrackArtist - Name"
	const desc = `
TrackArtist - Name

TrackArtist
- track.com/artist

Artwork by ArtworkArtist
- artwork.com/artist
`
	c := Collections{
		Artists: []*Artist{
			{"TrackArtist", []string{"track.com/artist"}},
			{"ArtworkArtist", []string{"artwork.com/artist"}},
		},
		Indexes: make(map[string]Collection),
	}

	b := VideoBuilder{
		Track: &Track{
			Title:   "Name",
			By:      "TrackArtist",
			Artists: []string{"TrackArtist"},
		},
		Art: &Artwork{
			Artist: "ArtworkArtist",
		},
		Format: &defaultConfig.VideoFormat,
	}

	t.Run("Title", func(t *testing.T) {
		s, err := b.Title()
		if err != nil {
			t.Error(err)
		}
		if s != title {
			t.Errorf("expected %s, got %s\n", title, s)
		}
	})

	t.Run("Desc", func(t *testing.T) {
		expect := desc[1:]
		s, err := b.Desc(&c)
		if err != nil {
			t.Error(err)
		}
		if s != expect {
			t.Errorf("\nexpected\n'%s'\ngot\n'%s'", expect, s)
		}
	})
}

func TestAdd(t *testing.T) {
	c := Collections{Indexes: make(map[string]Collection)}

	t.Run("Artwork", func(t *testing.T) {
		art := Artwork{Path: "/art", Artist: "artworkartist"}
		AddArtwork(&c, art)

		if _, ok := c.Find(art.UniqueId()); !ok {
			t.Errorf("did not insert %v", art)
		}
		if _, ok := c.Find(art.Artist); !ok {
			t.Errorf("did not insert %v", art.Artist)
		}
	})

	t.Run("Track", func(t *testing.T) {
		track := Track{Path: "/track", Artists: []string{"trackartist"}}
		AddTrack(&c, track)

		if _, ok := c.Find(track.UniqueId()); !ok {
			t.Errorf("did not insert %v", track)
		}
		if _, ok := c.Find(track.Artists[0]); !ok {
			t.Errorf("did not insert %v", track.Artists[0])
		}
	})
}

func TestInferArtists(t *testing.T) {
	tests := []struct {
		title  string
		artist string
		expect string
	}{
		{"", "A1", "A1"},
		{"Name", "A1 & AA2", "A1,AA2"},
		{"Name", "A1 X1 X A2 &A2", "A1 X1,A2 &A2"},
		{"Name ft. F1", "A1", "A1,F1"},
		{"Name feat. F1", "A1 x A2 X A3 A3 & A4", "A1,A2,A3 A3,A4,F1"},
		{"", "", ""},
	}

	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			a := inferArtists(tt.title, tt.artist, AddOptions{})
			got := strings.Join(a, ",")
			if tt.expect != got {
				t.Errorf("expected %s, got %s", tt.expect, got)
			}
		})
	}
}

func TestCollectionsFind(t *testing.T) {
	art := Artwork{Path: "/art"}
	track := Track{Path: "/track"}

	c := Collections{
		Tracks: []*Track{
			{Path: "/track1"},
		},
		Artwork: []*Artwork{
			{Path: "/art1"},
			{Path: "/art3"},
			&art,
		},
		Indexes: make(map[string]Collection),
	}

	t.Run("Exists", func(t *testing.T) {
		a, ok := c.Find(art.UniqueId())
		if !ok {
			t.Errorf("did not find %v in collection", art)
		}
		if a.(*Artwork) != &art {
			t.Errorf("expected %p, got %p", &art, a.(*Artwork))
		}
	})

	t.Run("New", func(t *testing.T) {
		_, ok := c.Find(track.UniqueId())
		if ok {
			t.Errorf("did not expect to find %v in collection", track)
		}
	})
}
