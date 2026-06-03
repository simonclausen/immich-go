package nextcloudmemories

import (
	"context"
	"fmt"
	"path"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/simulot/immich-go/internal/assets"
	"github.com/simulot/immich-go/internal/nextcloud"
	"golang.org/x/sync/errgroup"
)

type memoriesMetadataIndex struct {
	byPath map[string]*assets.Metadata
}

func newMemoriesMetadataIndex() *memoriesMetadataIndex {
	return &memoriesMetadataIndex{
		byPath: map[string]*assets.Metadata{},
	}
}

func (idx *memoriesMetadataIndex) Get(name string) (*assets.Metadata, bool) {
	if idx == nil {
		return nil, false
	}
	md, ok := idx.byPath[normalizeIndexedPath(name)]
	return md, ok
}

func (idx *memoriesMetadataIndex) Put(name string, md *assets.Metadata) {
	if idx == nil || md == nil {
		return
	}
	key := normalizeIndexedPath(name)
	if key == "" {
		return
	}
	idx.byPath[key] = md
}

func (nc *Command) ensureMetadataIndex(ctx context.Context) error {
	if nc.metadataIndex != nil {
		return nil
	}
	if nc.client == nil || nc.discovery == nil {
		return nil
	}

	index, err := nc.buildMetadataIndex(ctx)
	if err != nil {
		return err
	}
	nc.metadataIndex = index
	return nil
}

func (nc *Command) buildMetadataIndex(ctx context.Context) (*memoriesMetadataIndex, error) {
	photoByID := map[int]nextcloud.MemoriesPhoto{}
	query := nextcloud.MemoriesTimelineQuery{
		Recursive: true,
		Hidden:    true,
	}

	for _, root := range nc.selectedRoots {
		query.Folder = root

		days, err := nextcloud.GetMemoriesDays(ctx, nc.client, query)
		if err != nil {
			return nil, fmt.Errorf("failed to list Memories days for %q: %w", root, err)
		}

		dayIDs := make([]int, 0, len(days))
		for _, day := range days {
			dayIDs = append(dayIDs, day.DayID)
		}

		for start := 0; start < len(dayIDs); start += 100 {
			end := min(start+100, len(dayIDs))
			photos, err := nextcloud.GetMemoriesDay(ctx, nc.client, dayIDs[start:end], query)
			if err != nil {
				return nil, fmt.Errorf("failed to list Memories photos for %q: %w", root, err)
			}
			for _, photo := range photos {
				photoByID[photo.FileID] = mergeMemoriesPhoto(photoByID[photo.FileID], photo)
			}
		}
	}

	archiveQuery := nextcloud.MemoriesTimelineQuery{
		Recursive: true,
		Archive:   true,
		Hidden:    true,
	}
	archiveDays, err := nextcloud.GetMemoriesDays(ctx, nc.client, archiveQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to list Memories archive days: %w", err)
	}
	archiveDayIDs := make([]int, 0, len(archiveDays))
	for _, day := range archiveDays {
		archiveDayIDs = append(archiveDayIDs, day.DayID)
	}
	for start := 0; start < len(archiveDayIDs); start += 100 {
		end := min(start+100, len(archiveDayIDs))
		photos, err := nextcloud.GetMemoriesDay(ctx, nc.client, archiveDayIDs[start:end], archiveQuery)
		if err != nil {
			return nil, fmt.Errorf("failed to list Memories archived photos: %w", err)
		}
		for _, photo := range photos {
			photo.Archived = true
			photoByID[photo.FileID] = mergeMemoriesPhoto(photoByID[photo.FileID], photo)
		}
	}

	index := newMemoriesMetadataIndex()
	if len(photoByID) == 0 {
		return index, nil
	}

	ownerUID := strings.TrimSpace(nc.NextcloudUser)
	if nc.discovery.Describe.UID != nil && strings.TrimSpace(*nc.discovery.Describe.UID) != "" {
		ownerUID = strings.TrimSpace(*nc.discovery.Describe.UID)
	}

	infoQuery := nextcloud.MemoriesImageInfoQuery{
		Tags: nc.discovery.Config.SystemTagsEnabled,
	}
	if nc.SyncAlbums && nc.discovery.Config.AlbumsEnabled {
		infoQuery.Clusters = []string{"albums"}
	}

	var mu sync.Mutex
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(8)

	for _, photo := range photoByID {
		photo := photo
		group.Go(func() error {
			info, err := nextcloud.GetMemoriesImageInfo(groupCtx, nc.client, photo.FileID, infoQuery)
			if err != nil {
				return fmt.Errorf("failed to load Memories image info for file %d: %w", photo.FileID, err)
			}

			indexedPath := normalizeIndexedPath(info.FileName)
			if indexedPath == "" {
				return fmt.Errorf("Memories image info for file %d did not include a filename", photo.FileID)
			}
			if !isSelectedRootPath(indexedPath, nc.selectedRoots) {
				return nil
			}

			md := metadataFromMemories(photo, info, ownerUID)

			mu.Lock()
			index.Put(indexedPath, md)
			mu.Unlock()
			return nil
		})
	}

	if err := group.Wait(); err != nil {
		return nil, err
	}
	return index, nil
}

func metadataFromMemories(photo nextcloud.MemoriesPhoto, info *nextcloud.MemoriesImageInfo, ownerUID string) *assets.Metadata {
	md := &assets.Metadata{
		FileName:  info.Basename,
		Archived:  photo.Archived,
		Favorited: bool(photo.IsFavorite),
	}

	if info.DateTaken != 0 {
		md.DateTaken = time.Unix(info.DateTaken, 0).In(time.Local)
	} else if photo.DateTaken != 0 {
		md.DateTaken = time.Unix(photo.DateTaken, 0).In(time.Local)
	}

	if description := memoriesExifString(info.Exif, "Description"); description != "" {
		md.Description = description
	}
	if rating, ok := memoriesExifInt(info.Exif, "Rating"); ok {
		if rating < 0 {
			rating = 0
		}
		if rating > 5 {
			rating = 5
		}
		md.Rating = byte(rating)
	}
	if latitude, ok := memoriesExifFloat(info.Exif, "GPSLatitude"); ok {
		md.Latitude = latitude
	}
	if longitude, ok := memoriesExifFloat(info.Exif, "GPSLongitude"); ok {
		md.Longitude = longitude
	}

	if len(info.Tags) > 0 {
		tags := make([]string, 0, len(info.Tags))
		for _, tag := range info.Tags {
			tag = strings.TrimSpace(tag)
			if tag == "" {
				continue
			}
			tags = append(tags, tag)
		}
		slices.Sort(tags)
		for _, tag := range tags {
			md.AddTag(tag)
		}
	}

	if len(info.Clusters.Albums) > 0 {
		seenAlbums := map[string]struct{}{}
		for _, album := range info.Clusters.Albums {
			title := memoriesAlbumTitle(album, ownerUID)
			if title == "" {
				continue
			}
			if _, ok := seenAlbums[title]; ok {
				continue
			}
			seenAlbums[title] = struct{}{}
			md.Albums = append(md.Albums, assets.NewAlbum("", title, ""))
		}
		slices.SortFunc(md.Albums, func(a, b assets.Album) int {
			return strings.Compare(a.Title, b.Title)
		})
	}

	return md
}

func mergeMemoriesPhoto(current, incoming nextcloud.MemoriesPhoto) nextcloud.MemoriesPhoto {
	if current.FileID == 0 {
		return incoming
	}
	if current.Basename == "" {
		current.Basename = incoming.Basename
	}
	if current.MimeType == "" {
		current.MimeType = incoming.MimeType
	}
	if current.DayID == 0 {
		current.DayID = incoming.DayID
	}
	if current.DateTaken == 0 {
		current.DateTaken = incoming.DateTaken
	}
	current.Archived = current.Archived || incoming.Archived
	current.IsFavorite = current.IsFavorite || incoming.IsFavorite
	current.IsHidden = current.IsHidden || incoming.IsHidden
	return current
}

func normalizeIndexedPath(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	cleaned := path.Clean("/" + strings.TrimPrefix(name, "/"))
	if cleaned == "/" {
		return ""
	}
	return strings.TrimPrefix(cleaned, "/")
}

func isSelectedRootPath(name string, roots []string) bool {
	if len(roots) == 0 {
		return true
	}
	for _, root := range roots {
		rootPath := timelineRootToFSPath(root)
		if rootPath == "." || name == rootPath || strings.HasPrefix(name, rootPath+"/") {
			return true
		}
	}
	return false
}

func memoriesExifString(exif map[string]any, key string) string {
	if exif == nil {
		return ""
	}
	raw, ok := exif[key]
	if !ok || raw == nil {
		return ""
	}
	value := strings.TrimSpace(fmt.Sprint(raw))
	if strings.EqualFold(value, "<nil>") {
		return ""
	}
	return value
}

func memoriesExifInt(exif map[string]any, key string) (int, bool) {
	value := memoriesExifString(exif, key)
	if value == "" {
		return 0, false
	}
	parsed, err := strconv.Atoi(value)
	if err == nil {
		return parsed, true
	}
	floatValue, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, false
	}
	return int(floatValue), true
}

func memoriesExifFloat(exif map[string]any, key string) (float64, bool) {
	value := memoriesExifString(exif, key)
	if value == "" {
		return 0, false
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, false
	}
	return parsed, true
}

func memoriesAlbumTitle(album nextcloud.MemoriesAlbum, ownerUID string) string {
	name := strings.TrimSpace(album.Name)
	if name == "" {
		return ""
	}
	if album.User == "" || album.User == ownerUID {
		return name
	}
	sharedBy := strings.TrimSpace(album.UserDisplay)
	if sharedBy == "" {
		sharedBy = strings.TrimSpace(album.User)
	}
	if sharedBy == "" {
		return name
	}
	return fmt.Sprintf("%s (shared by %s)", name, sharedBy)
}