package deezer

import (
	"testing"
)

// These tests exercise the conversion helpers and the URI driver's pure string
// functions, which need no network. HTTP behaviour is covered in deezer_test.go.

func TestDomainInfo(t *testing.T) {
	info := Domain{}.Info()
	if info.Scheme != "deezer" {
		t.Errorf("Scheme = %q, want deezer", info.Scheme)
	}
	if len(info.Hosts) == 0 || info.Hosts[0] != Host {
		t.Errorf("Hosts = %v, want [%s]", info.Hosts, Host)
	}
	if info.Identity.Binary != "deezer" {
		t.Errorf("Identity.Binary = %q, want deezer", info.Identity.Binary)
	}
}

func TestToTrack(t *testing.T) {
	w := wireTrack{
		ID:       3135556,
		Title:    "Harder, Better, Faster, Stronger",
		Duration: 224,
		Preview:  "https://cdn.deezer.com/preview/abc.mp3",
		Rank:     999901,
	}
	w.Artist.Name = "Daft Punk"
	w.Album.Title = "Discovery"

	got := toTrack(w)
	if got.ID != 3135556 {
		t.Errorf("ID = %d, want 3135556", got.ID)
	}
	if got.Title != "Harder, Better, Faster, Stronger" {
		t.Errorf("Title = %q", got.Title)
	}
	if got.Artist != "Daft Punk" {
		t.Errorf("Artist = %q, want Daft Punk", got.Artist)
	}
	if got.Album != "Discovery" {
		t.Errorf("Album = %q, want Discovery", got.Album)
	}
	if got.Duration != 224 {
		t.Errorf("Duration = %d, want 224", got.Duration)
	}
	if got.Preview != "https://cdn.deezer.com/preview/abc.mp3" {
		t.Errorf("Preview = %q", got.Preview)
	}
	if got.Rank != 999901 {
		t.Errorf("Rank = %d, want 999901", got.Rank)
	}
}

func TestToArtist(t *testing.T) {
	w := wireArtist{
		ID:            27,
		Name:          "Daft Punk",
		NbAlbum:       31,
		NbFan:         7019308,
		PictureMedium: "https://e-cdns-images.dzcdn.net/artist/pic.jpg",
	}

	got := toArtist(w)
	if got.ID != 27 {
		t.Errorf("ID = %d, want 27", got.ID)
	}
	if got.Name != "Daft Punk" {
		t.Errorf("Name = %q, want Daft Punk", got.Name)
	}
	if got.Albums != 31 {
		t.Errorf("Albums = %d, want 31", got.Albums)
	}
	if got.Fans != 7019308 {
		t.Errorf("Fans = %d, want 7019308", got.Fans)
	}
	if got.PictureURL != "https://e-cdns-images.dzcdn.net/artist/pic.jpg" {
		t.Errorf("PictureURL = %q", got.PictureURL)
	}
}

func TestToAlbum(t *testing.T) {
	w := wireAlbum{
		ID:          302127,
		Title:       "Discovery",
		NbTracks:    14,
		ReleaseDate: "2001-02-26",
		CoverMedium: "https://e-cdns-images.dzcdn.net/cover/disc.jpg",
	}
	w.Artist.Name = "Daft Punk"
	w.Genres.Data = []struct {
		Name string `json:"name"`
	}{
		{Name: "Electronic"},
		{Name: "Dance"},
	}
	wt := wireTrack{ID: 1, Title: "One More Time", Duration: 320}
	wt.Artist.Name = "Daft Punk"
	w.Tracks.Data = []wireTrack{wt}

	got := toAlbum(w)
	if got.ID != 302127 {
		t.Errorf("ID = %d, want 302127", got.ID)
	}
	if got.Title != "Discovery" {
		t.Errorf("Title = %q, want Discovery", got.Title)
	}
	if got.Artist != "Daft Punk" {
		t.Errorf("Artist = %q, want Daft Punk", got.Artist)
	}
	if got.Tracks != 14 {
		t.Errorf("Tracks = %d, want 14", got.Tracks)
	}
	if got.ReleaseDate != "2001-02-26" {
		t.Errorf("ReleaseDate = %q", got.ReleaseDate)
	}
	if len(got.Genres) != 2 || got.Genres[0] != "Electronic" || got.Genres[1] != "Dance" {
		t.Errorf("Genres = %v, want [Electronic Dance]", got.Genres)
	}
	if len(got.TrackList) != 1 || got.TrackList[0].Title != "One More Time" {
		t.Errorf("TrackList = %v", got.TrackList)
	}
	if got.CoverURL != "https://e-cdns-images.dzcdn.net/cover/disc.jpg" {
		t.Errorf("CoverURL = %q", got.CoverURL)
	}
}

func TestLocate(t *testing.T) {
	cases := []struct {
		uriType string
		id      string
		want    string
	}{
		{"track", "3135556", baseURL + "/track/3135556"},
		{"artist", "27", baseURL + "/artist/27"},
		{"album", "302127", baseURL + "/album/302127"},
	}
	for _, tc := range cases {
		got, err := Domain{}.Locate(tc.uriType, tc.id)
		if err != nil {
			t.Errorf("Locate(%q, %q) error: %v", tc.uriType, tc.id, err)
			continue
		}
		if got != tc.want {
			t.Errorf("Locate(%q, %q) = %q, want %q", tc.uriType, tc.id, got, tc.want)
		}
	}
}

func TestLocateUnknownType(t *testing.T) {
	_, err := Domain{}.Locate("playlist", "123")
	if err == nil {
		t.Error("expected error for unknown type, got nil")
	}
}
