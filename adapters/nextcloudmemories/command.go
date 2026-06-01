package nextcloudmemories

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/simulot/immich-go/adapters"
	"github.com/simulot/immich-go/app"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var ErrNotImplemented = errors.New("Nextcloud Memories import is not implemented yet")

// Command holds the CLI state for the Nextcloud Memories source scaffold.
type Command struct {
	NextcloudURL           string
	NextcloudUser          string
	NextcloudPassword      string
	NextcloudSkipVerifySSL bool
	NextcloudClientTimeout time.Duration
	DiscoverOnly           bool
	TimelineRoots          []string
	SyncAlbums             bool
	AllowUnindexed         bool

	app *app.Application
}

func (nc *Command) RegisterFlags(flags *pflag.FlagSet) {
	flags.StringVar(&nc.NextcloudURL, "nextcloud-url", "", "Nextcloud base URL")
	flags.StringVar(&nc.NextcloudUser, "nextcloud-user", "", "Nextcloud username")
	flags.StringVar(&nc.NextcloudPassword, "nextcloud-password", "", "Nextcloud password or app password")
	flags.BoolVar(&nc.NextcloudSkipVerifySSL, "nextcloud-skip-verify-ssl", false, "Skip TLS verification for the source Nextcloud server")
	flags.DurationVar(&nc.NextcloudClientTimeout, "nextcloud-client-timeout", 5*time.Minute, "Timeout for source Nextcloud API calls")
	flags.BoolVar(&nc.DiscoverOnly, "discover-only", false, "Print detected Memories configuration and exit")
	flags.StringSliceVar(&nc.TimelineRoots, "timeline-root", nil, "Limit the import to configured Memories timeline roots. Can be specified multiple times")
	flags.BoolVar(&nc.SyncAlbums, "sync-albums", true, "Recreate Memories albums in Immich")
	flags.BoolVar(&nc.AllowUnindexed, "allow-unindexed", false, "Continue even if the source Memories library appears partially indexed")
}

// NewFromNextcloudMemoriesCommand creates a hidden command scaffold for the planned
// Nextcloud Memories source importer. The command is callable for UX iteration but
// remains hidden until source discovery and browsing are implemented.
func NewFromNextcloudMemoriesCommand(ctx context.Context, parent *cobra.Command, app *app.Application, runner adapters.Runner) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "from-nextcloud-memories [flags]",
		Aliases: []string{"from-nc-memories"},
		Short:   "Upload photos from a Nextcloud Memories library",
		Long: strings.TrimSpace(`Import photos and videos from a Nextcloud instance that uses the Memories app.

This source is intended to migrate the configured Memories library for a user account,
not arbitrary Nextcloud storage paths. The planned workflow is:

1. Authenticate to Nextcloud.
2. Discover the user's effective Memories configuration.
3. Resolve the configured timeline roots.
4. Import assets and supported metadata from that scope.

This command scaffold is intentionally hidden while the source implementation is still
being built. The current branch uses it to iterate on the UX and flag contract safely.`),
		Example: strings.TrimSpace(`  immich-go upload from-nextcloud-memories \
    --nextcloud-url=https://cloud.example.com \
    --nextcloud-user=alice \
    --nextcloud-password="$NEXTCLOUD_APP_PASSWORD" \
    --discover-only

  immich-go upload from-nextcloud-memories \
    --nextcloud-url=https://cloud.example.com \
    --nextcloud-user=alice \
    --nextcloud-password="$NEXTCLOUD_APP_PASSWORD" \
    --timeline-root=/Photos \
    --server=http://immich.example.com:2283 \
    --api-key="$IMMICH_API_KEY"`),
		Args:   cobra.NoArgs,
		Hidden: true,
	}
	cmd.SetContext(ctx)

	nc := &Command{app: app}
	nc.RegisterFlags(cmd.Flags())

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return nc.Run(cmd, runner)
	}

	return cmd
}

// Run validates the planned UX surface and fails explicitly until the source-side
// discovery and browsing layers are implemented.
func (nc *Command) Run(cmd *cobra.Command, runner adapters.Runner) error {
	if err := nc.validate(); err != nil {
		return err
	}

	if nc.app != nil {
		nc.app.Log().Message("Nextcloud Memories command scaffold invoked (discover-only=%t)", nc.DiscoverOnly)
	}

	if nc.DiscoverOnly {
		return fmt.Errorf("%w: source discovery is not implemented yet", ErrNotImplemented)
	}

	_ = runner
	return fmt.Errorf("%w: asset browsing is not implemented yet", ErrNotImplemented)
}

func (nc *Command) validate() error {
	var joinedErr error

	nc.NextcloudURL = strings.TrimSpace(nc.NextcloudURL)
	nc.NextcloudUser = strings.TrimSpace(nc.NextcloudUser)
	nc.NextcloudPassword = strings.TrimSpace(nc.NextcloudPassword)
	nc.TimelineRoots = normalizeTimelineRoots(nc.TimelineRoots)

	if nc.NextcloudURL == "" {
		joinedErr = errors.Join(joinedErr, errors.New("missing the parameter --nextcloud-url, Nextcloud base URL"))
	} else {
		parsedURL, err := url.Parse(nc.NextcloudURL)
		if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
			joinedErr = errors.Join(joinedErr, fmt.Errorf("invalid --nextcloud-url %q: must include scheme and host", nc.NextcloudURL))
		} else {
			nc.NextcloudURL = strings.TrimRight(parsedURL.String(), "/")
		}
	}
	if nc.NextcloudUser == "" {
		joinedErr = errors.Join(joinedErr, errors.New("missing the parameter --nextcloud-user, Nextcloud username"))
	}
	if nc.NextcloudPassword == "" {
		joinedErr = errors.Join(joinedErr, errors.New("missing the parameter --nextcloud-password, Nextcloud password or app password"))
	}
	if nc.NextcloudClientTimeout <= 0 {
		joinedErr = errors.Join(joinedErr, errors.New("invalid --nextcloud-client-timeout: must be greater than 0"))
	}
	for _, root := range nc.TimelineRoots {
		if !strings.HasPrefix(root, "/") {
			joinedErr = errors.Join(joinedErr, fmt.Errorf("invalid --timeline-root %q: must start with /", root))
		}
	}

	return joinedErr
}

func normalizeTimelineRoots(roots []string) []string {
	normalized := make([]string, 0, len(roots))
	seen := map[string]struct{}{}

	for _, root := range roots {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		if root != "/" {
			root = strings.TrimRight(root, "/")
		}
		if root == "" {
			root = "/"
		}
		if _, ok := seen[root]; ok {
			continue
		}
		seen[root] = struct{}{}
		normalized = append(normalized, root)
	}

	return normalized
}
