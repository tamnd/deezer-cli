// Package deezer is the library behind the deezer command line:
// the HTTP client, request shaping, and the typed data models for the
// Deezer public music API (https://api.deezer.com/).
//
// The Client here is the spine every command shares. It sets a real
// User-Agent, paces requests so a busy session stays polite, and retries the
// transient failures (429 and 5xx) that any public API throws under load.
package deezer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// DefaultUserAgent identifies the client to Deezer. A real, honest
// User-Agent is both polite and the thing most likely to keep you unblocked.
const DefaultUserAgent = "deezer-cli/0.1 (tamnd87@gmail.com)"

// Host is the site this client talks to.
const Host = "api.deezer.com"

// baseURL is the root every request is built from.
const baseURL = "https://api.deezer.com"

// defaultTimeout is the HTTP request timeout.
const defaultTimeout = 15 * time.Second

// defaultRate is the minimum gap between requests.
const defaultRate = 200 * time.Millisecond

// maxRetries is the number of retries on transient errors.
const maxRetries = 3

// Client talks to the Deezer API over HTTP.
type Client struct {
	HTTP      *http.Client
	UserAgent string
	// Rate is the minimum gap between requests. Zero means no pacing.
	Rate    time.Duration
	Retries int

	baseURL string
	mu      sync.Mutex
	last    time.Time
}

// NewClient returns a Client with sensible defaults.
func NewClient() *Client {
	return &Client{
		HTTP:      &http.Client{Timeout: defaultTimeout},
		UserAgent: DefaultUserAgent,
		Rate:      defaultRate,
		Retries:   maxRetries,
		baseURL:   baseURL,
	}
}

// get fetches the given path with optional query parameters and returns the body bytes.
func (c *Client) get(ctx context.Context, path string, q url.Values) ([]byte, error) {
	u := c.baseURL + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	var lastErr error
	for attempt := 0; attempt <= c.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, u)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", path, lastErr)
}

func (c *Client) do(ctx context.Context, rawURL string) (body []byte, retry bool, err error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, true, err
	}
	return b, false, nil
}

// pace blocks until at least Rate has passed since the previous request.
func (c *Client) pace() {
	if c.Rate <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if wait := c.Rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}

// --- Public output types ---

// Track is a Deezer track (song).
type Track struct {
	ID       int    `kit:"id" json:"id"`
	Title    string `json:"title"`
	Artist   string `json:"artist"`
	Album    string `json:"album"`
	Duration int    `json:"duration_sec"`
	Rank     int    `json:"rank"`
}

// Artist is a Deezer artist.
type Artist struct {
	ID     int    `kit:"id" json:"id"`
	Name   string `json:"name"`
	Albums int    `json:"albums"`
	Fans   int    `json:"fans"`
	URL    string `json:"url"`
}

// Album is a Deezer album.
type Album struct {
	ID       int    `kit:"id" json:"id"`
	Title    string `json:"title"`
	Artist   string `json:"artist"`
	Tracks   int    `json:"tracks"`
	Duration int    `json:"duration_sec"`
	Released string `json:"released"`
	Label    string `json:"label"`
	Fans     int    `json:"fans"`
	Genres   string `json:"genres"`
}

// --- Wire types (unexported) ---

type wireTrack struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	Artist struct {
		Name string `json:"name"`
	} `json:"artist"`
	Album struct {
		Title string `json:"title"`
	} `json:"album"`
	Duration int `json:"duration"`
	Rank     int `json:"rank"`
}

type wireArtist struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	NbAlbum int    `json:"nb_album"`
	NbFan   int    `json:"nb_fan"`
	Link    string `json:"link"`
}

type wireAlbum struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	Artist struct {
		Name string `json:"name"`
	} `json:"artist"`
	NbTracks    int    `json:"nb_tracks"`
	Duration    int    `json:"duration"`
	ReleaseDate string `json:"release_date"`
	Label       string `json:"label"`
	Fans        int    `json:"fans"`
	Genres      struct {
		Data []struct {
			Name string `json:"name"`
		} `json:"data"`
	} `json:"genres"`
}

// --- Conversion helpers ---

func toTrack(w wireTrack) Track {
	return Track{
		ID:       w.ID,
		Title:    w.Title,
		Artist:   w.Artist.Name,
		Album:    w.Album.Title,
		Duration: w.Duration,
		Rank:     w.Rank,
	}
}

func toArtist(w wireArtist) Artist {
	return Artist{
		ID:     w.ID,
		Name:   w.Name,
		Albums: w.NbAlbum,
		Fans:   w.NbFan,
		URL:    w.Link,
	}
}

func toAlbum(w wireAlbum) Album {
	genres := make([]string, 0, len(w.Genres.Data))
	for _, g := range w.Genres.Data {
		genres = append(genres, g.Name)
	}
	return Album{
		ID:       w.ID,
		Title:    w.Title,
		Artist:   w.Artist.Name,
		Tracks:   w.NbTracks,
		Duration: w.Duration,
		Released: w.ReleaseDate,
		Label:    w.Label,
		Fans:     w.Fans,
		Genres:   strings.Join(genres, ", "),
	}
}

// --- API methods ---

// Search searches for tracks matching query.
func (c *Client) Search(ctx context.Context, query string, limit int) ([]Track, error) {
	q := url.Values{"q": {query}, "limit": {strconv.Itoa(limit)}}
	body, err := c.get(ctx, "/search", q)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data []wireTrack `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode search: %w", err)
	}
	out := make([]Track, len(resp.Data))
	for i, w := range resp.Data {
		out[i] = toTrack(w)
	}
	return out, nil
}

// GetArtist fetches artist info by ID.
func (c *Client) GetArtist(ctx context.Context, id int) (*Artist, error) {
	body, err := c.get(ctx, fmt.Sprintf("/artist/%d", id), nil)
	if err != nil {
		return nil, err
	}
	var wa wireArtist
	if err := json.Unmarshal(body, &wa); err != nil {
		return nil, fmt.Errorf("decode artist: %w", err)
	}
	a := toArtist(wa)
	return &a, nil
}

// SearchArtists searches for artists matching query.
func (c *Client) SearchArtists(ctx context.Context, query string, limit int) ([]Artist, error) {
	q := url.Values{"q": {query}, "limit": {strconv.Itoa(limit)}}
	body, err := c.get(ctx, "/search/artist", q)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data []wireArtist `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode search/artist: %w", err)
	}
	out := make([]Artist, len(resp.Data))
	for i, w := range resp.Data {
		out[i] = toArtist(w)
	}
	return out, nil
}

// GetAlbum fetches album details by ID.
func (c *Client) GetAlbum(ctx context.Context, id int) (*Album, error) {
	body, err := c.get(ctx, fmt.Sprintf("/album/%d", id), nil)
	if err != nil {
		return nil, err
	}
	var wa wireAlbum
	if err := json.Unmarshal(body, &wa); err != nil {
		return nil, fmt.Errorf("decode album: %w", err)
	}
	a := toAlbum(wa)
	return &a, nil
}

// GetTrack fetches a single track by ID.
func (c *Client) GetTrack(ctx context.Context, id int) (*Track, error) {
	body, err := c.get(ctx, fmt.Sprintf("/track/%d", id), nil)
	if err != nil {
		return nil, err
	}
	var wt wireTrack
	if err := json.Unmarshal(body, &wt); err != nil {
		return nil, fmt.Errorf("decode track: %w", err)
	}
	t := toTrack(wt)
	return &t, nil
}
