package deezer

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/tamnd/any-cli/kit"
)

// domain.go exposes deezer as a kit Domain: a driver that a multi-domain
// host (ant) enables with a single blank import,
//
//	import _ "github.com/tamnd/deezer-cli/deezer"
//
// exactly as a database/sql program enables a driver with `import _
// "github.com/lib/pq"`. The init below registers it; the host then routes
// deezer:// URIs to the operations Register installs. The same Domain also
// builds the standalone deezer binary (see cmd/deezer/main.go).
func init() { kit.Register(Domain{}) }

// Domain is the deezer driver. It carries no state; the per-run client is
// built by the factory Register hands kit.
type Domain struct{}

// Info describes the scheme, the hostnames a pasted link is matched against,
// and the identity reused for the binary's help and version.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "deezer",
		Hosts:  []string{Host},
		Identity: kit.Identity{
			Binary: "deezer",
			Short:  "A command line for the Deezer music API.",
			Long: `A command line for the Deezer public music API.

deezer reads public Deezer data over plain HTTPS with no API key required,
shapes it into clean records, and prints output that pipes into the rest of
your tools.`,
			Site: Host,
			Repo: "https://github.com/tamnd/deezer-cli",
		},
	}
}

// Register installs the client factory and every operation onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	kit.Handle(app, kit.OpMeta{
		Name:    "search",
		Group:   "tracks",
		Summary: "Search for tracks by query",
		Args:    []kit.Arg{{Name: "query", Help: "search terms", Variadic: true}},
	}, searchOp)

	kit.Handle(app, kit.OpMeta{
		Name:    "artist",
		Group:   "library",
		Summary: "Get artist info by ID",
		Args:    []kit.Arg{{Name: "id", Help: "artist ID"}},
	}, artistOp)

	kit.Handle(app, kit.OpMeta{
		Name:    "artist-search",
		Group:   "library",
		Summary: "Search for artists by query",
		Args:    []kit.Arg{{Name: "query", Help: "search terms", Variadic: true}},
	}, artistSearchOp)

	kit.Handle(app, kit.OpMeta{
		Name:    "album",
		Group:   "library",
		Summary: "Get album details by ID",
		Args:    []kit.Arg{{Name: "id", Help: "album ID"}},
	}, albumOp)

	kit.Handle(app, kit.OpMeta{
		Name:    "track",
		Group:   "library",
		Summary: "Get a single track by ID",
		Args:    []kit.Arg{{Name: "id", Help: "track ID"}},
	}, trackOp)
}

// newClient builds the client from the host-resolved config.
func newClient(_ context.Context, cfg kit.Config) (any, error) {
	c := NewClient()
	if cfg.UserAgent != "" {
		c.UserAgent = cfg.UserAgent
	}
	if cfg.Rate > 0 {
		c.Rate = cfg.Rate
	}
	if cfg.Retries > 0 {
		c.Retries = cfg.Retries
	}
	if cfg.Timeout > 0 {
		c.HTTP.Timeout = cfg.Timeout
	}
	return c, nil
}

// --- input structs ---

type searchIn struct {
	Query  []string `kit:"arg,variadic" help:"search terms"`
	Limit  int      `kit:"flag,inherit" help:"max results"`
	Client *Client  `kit:"inject"`
}

type artistIn struct {
	ID     string  `kit:"arg" help:"artist ID"`
	Client *Client `kit:"inject"`
}

type artistSearchIn struct {
	Query  []string `kit:"arg,variadic" help:"search terms"`
	Limit  int      `kit:"flag,inherit" help:"max results"`
	Client *Client  `kit:"inject"`
}

type albumIn struct {
	ID     string  `kit:"arg" help:"album ID"`
	Client *Client `kit:"inject"`
}

type trackIn struct {
	ID     string  `kit:"arg" help:"track ID"`
	Client *Client `kit:"inject"`
}

// --- handlers ---

func searchOp(ctx context.Context, in searchIn, emit func(*Track) error) error {
	if len(in.Query) == 0 {
		return fmt.Errorf("query is required")
	}
	limit := in.Limit
	if limit <= 0 {
		limit = 20
	}
	tracks, err := in.Client.Search(ctx, strings.Join(in.Query, " "), limit)
	if err != nil {
		return err
	}
	for i := range tracks {
		if err := emit(&tracks[i]); err != nil {
			return err
		}
	}
	return nil
}

func artistOp(ctx context.Context, in artistIn, emit func(*Artist) error) error {
	if in.ID == "" {
		return fmt.Errorf("id is required")
	}
	id, err := strconv.Atoi(in.ID)
	if err != nil {
		return fmt.Errorf("invalid id %q: %w", in.ID, err)
	}
	a, err := in.Client.GetArtist(ctx, id)
	if err != nil {
		return err
	}
	return emit(a)
}

func artistSearchOp(ctx context.Context, in artistSearchIn, emit func(*Artist) error) error {
	if len(in.Query) == 0 {
		return fmt.Errorf("query is required")
	}
	limit := in.Limit
	if limit <= 0 {
		limit = 10
	}
	artists, err := in.Client.SearchArtists(ctx, strings.Join(in.Query, " "), limit)
	if err != nil {
		return err
	}
	for i := range artists {
		if err := emit(&artists[i]); err != nil {
			return err
		}
	}
	return nil
}

func albumOp(ctx context.Context, in albumIn, emit func(*Album) error) error {
	if in.ID == "" {
		return fmt.Errorf("id is required")
	}
	id, err := strconv.Atoi(in.ID)
	if err != nil {
		return fmt.Errorf("invalid id %q: %w", in.ID, err)
	}
	a, err := in.Client.GetAlbum(ctx, id)
	if err != nil {
		return err
	}
	return emit(a)
}

func trackOp(ctx context.Context, in trackIn, emit func(*Track) error) error {
	if in.ID == "" {
		return fmt.Errorf("id is required")
	}
	id, err := strconv.Atoi(in.ID)
	if err != nil {
		return fmt.Errorf("invalid id %q: %w", in.ID, err)
	}
	t, err := in.Client.GetTrack(ctx, id)
	if err != nil {
		return err
	}
	return emit(t)
}

// Classify and Locate satisfy the kit.Domain interface for URI routing.
func (Domain) Classify(input string) (uriType, id string, err error) {
	return "track", input, nil
}

func (Domain) Locate(uriType, id string) (string, error) {
	switch uriType {
	case "track":
		return baseURL + "/track/" + id, nil
	case "artist":
		return baseURL + "/artist/" + id, nil
	case "album":
		return baseURL + "/album/" + id, nil
	}
	return "", fmt.Errorf("deezer: unknown resource type %q", uriType)
}
