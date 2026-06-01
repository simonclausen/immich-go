package nextcloudmemories

import (
	"context"
	"errors"
	"io/fs"
	"path"
	"strings"

	"github.com/simulot/immich-go/adapters/shared"
	"github.com/simulot/immich-go/internal/assets"
	"github.com/simulot/immich-go/internal/fileevent"
	"github.com/simulot/immich-go/internal/filenames"
	"github.com/simulot/immich-go/internal/filetypes"
	"github.com/simulot/immich-go/internal/fshelper"
	"github.com/simulot/immich-go/internal/namematcher"
)

var defaultBannedFiles = namematcher.MustList(shared.DefaultBannedFiles...)

func (nc *Command) Browse(ctx context.Context) chan *assets.Group {
	gOut := make(chan *assets.Group)

	go func() {
		defer close(gOut)

		err := nc.browse(ctx, gOut)
		if nc.app != nil {
			_ = nc.app.ProcessError(err)
		}
	}()

	return gOut
}

func (nc *Command) browse(ctx context.Context, gOut chan<- *assets.Group) error {
	if nc.sourceFS == nil {
		return errors.New("Nextcloud Memories source is not initialized")
	}
	if len(nc.selectedRoots) == 0 {
		return errors.New("Nextcloud Memories import has no selected timeline roots")
	}

	supportedMedia := filetypes.DefaultSupportedMedia
	infoCollector := filenames.NewInfoCollector(nil, supportedMedia)
	processor := nc.fileProcessor()
	if nc.app != nil {
		supportedMedia = nc.app.GetSupportedMedia()
		infoCollector = filenames.NewInfoCollector(nc.app.GetTZ(), supportedMedia)
	}

	seen := map[string]struct{}{}
	for _, root := range nc.selectedRoots {
		walkRoot := timelineRootToFSPath(root)
		err := fs.WalkDir(nc.sourceFS, walkRoot, func(name string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			if entry.IsDir() {
				if matchesBanned(defaultBannedFiles, name, true) {
					return fs.SkipDir
				}
				return nil
			}

			if matchesBanned(defaultBannedFiles, name, false) {
				if processor != nil {
					processor.RecordNonAsset(ctx, fshelper.FSName(nc.sourceFS, name), 0, fileevent.DiscoveredBanned, "reason", "banned file")
				}
				return nil
			}

			if _, ok := seen[name]; ok {
				return nil
			}
			seen[name] = struct{}{}

			if supportedMedia.IsUseLess(name) {
				if processor != nil {
					processor.RecordNonAsset(ctx, fshelper.FSName(nc.sourceFS, name), 0, fileevent.DiscoveredUnknown, "reason", "useless file")
				}
				return nil
			}

			info, err := entry.Info()
			if err != nil {
				if processor != nil {
					processor.RecordNonAsset(ctx, fshelper.FSName(nc.sourceFS, name), 0, fileevent.ErrorFileAccess, "error", err.Error())
				}
				return nil
			}

			mediaType := supportedMedia.TypeFromExt(path.Ext(name))
			switch mediaType {
			case filetypes.TypeSidecar:
				if processor != nil {
					processor.RecordNonAsset(ctx, fshelper.FSName(nc.sourceFS, name), info.Size(), fileevent.DiscoveredSidecar)
				}
				return nil
			case filetypes.TypeImage, filetypes.TypeVideo:
			default:
				if processor != nil {
					processor.RecordNonAsset(ctx, fshelper.FSName(nc.sourceFS, name), info.Size(), fileevent.DiscoveredUnsupported, "reason", "unsupported file type")
				}
				return nil
			}

			asset := nc.assetFromInfo(name, info, infoCollector)
			if processor != nil {
				discoveryCode := fileevent.DiscoveredImage
				if mediaType == filetypes.TypeVideo {
					discoveryCode = fileevent.DiscoveredVideo
				}
				processor.RecordAssetDiscovered(ctx, asset.File, info.Size(), discoveryCode)
			}

			select {
			case gOut <- assets.NewGroup(assets.GroupByNone, asset):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func (nc *Command) assetFromInfo(name string, info fs.FileInfo, infoCollector *filenames.InfoCollector) *assets.Asset {
	asset := &assets.Asset{
		File:             fshelper.FSName(nc.sourceFS, name),
		FileSize:         int(info.Size()),
		FileDate:         info.ModTime(),
		OriginalFileName: path.Base(name),
	}
	asset.SetNameInfo(infoCollector.GetInfo(asset.OriginalFileName))
	return asset
}

func (nc *Command) fileProcessor() interface {
	RecordAssetDiscovered(context.Context, fshelper.FSAndName, int64, fileevent.Code)
	RecordNonAsset(context.Context, fshelper.FSAndName, int64, fileevent.Code, ...any)
} {
	if nc.app == nil {
		return nil
	}
	return nc.app.FileProcessor()
}

func timelineRootToFSPath(root string) string {
	root = strings.TrimSpace(root)
	if root == "" || root == "/" {
		return "."
	}
	return strings.TrimPrefix(root, "/")
}

func matchesBanned(list namematcher.List, name string, isDir bool) bool {
	trimmed := strings.TrimSuffix(name, "/")
	if isDir {
		if list.MatchDir(name) {
			return true
		}
		if trimmed != name && list.MatchDir(trimmed) {
			return true
		}
		return false
	}
	if list.MatchFile(name) {
		return true
	}
	if trimmed != name && list.MatchFile(trimmed) {
		return true
	}
	return false
}
