package nextcloud

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	pathpkg "path"
	"strconv"
	"strings"
	"time"
)

var ErrSearchUnsupported = errors.New("Nextcloud WebDAV SEARCH is not supported")

type SearchEntry struct {
	Path        string
	Info        fs.FileInfo
	ContentType string
	ETag        string
	FileID      string
	Checksums   []string
}

type searchMultiStatus struct {
	Responses []searchResponse `xml:"response"`
}

type searchResponse struct {
	Href      string           `xml:"href"`
	PropStats []searchPropStat `xml:"propstat"`
}

type searchPropStat struct {
	Status string     `xml:"status"`
	Prop   searchProp `xml:"prop"`
}

type searchProp struct {
	DisplayName   string             `xml:"displayname"`
	ContentType   string             `xml:"getcontenttype"`
	ETag          string             `xml:"getetag"`
	ContentLength string             `xml:"getcontentlength"`
	LastModified  string             `xml:"getlastmodified"`
	ResourceType  searchResourceType `xml:"resourcetype"`
	FileID        string             `xml:"fileid"`
	Checksums     []string           `xml:"checksums>checksum"`
}

type searchResourceType struct {
	Collection *struct{} `xml:"collection"`
}

type searchFileInfo struct {
	name    string
	size    int64
	modTime time.Time
	dir     bool
}

func (s searchFileInfo) Name() string { return s.name }
func (s searchFileInfo) Size() int64  { return s.size }
func (s searchFileInfo) Mode() fs.FileMode {
	if s.dir {
		return fs.ModeDir | 0o755
	}
	return 0o644
}
func (s searchFileInfo) ModTime() time.Time { return s.modTime }
func (s searchFileInfo) IsDir() bool        { return s.dir }
func (s searchFileInfo) Sys() any           { return nil }

func (c *Client) SearchFiles(ctx context.Context, scopePath string) ([]SearchEntry, error) {
	scopePath = strings.TrimSpace(scopePath)
	if scopePath == "" {
		return nil, errors.New("missing Nextcloud DAV search scope")
	}
	if !strings.HasPrefix(scopePath, "/") {
		scopePath = "/" + scopePath
	}

	body, err := buildSearchRequestBody(scopePath)
	if err != nil {
		return nil, err
	}

	searchURL := *c.davRootURL
	if !strings.HasSuffix(searchURL.Path, "/") {
		searchURL.Path += "/"
	}

	req, err := http.NewRequestWithContext(ctx, "SEARCH", searchURL.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.username, c.password)
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	req.Header.Set("Accept", "text/xml, application/xml")

	resp, err := c.davHTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusMultiStatus:
		return decodeSearchResponse(resp.Body, c.davRootURL.Path)
	case http.StatusMethodNotAllowed, http.StatusNotImplemented:
		return nil, ErrSearchUnsupported
	default:
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("Nextcloud DAV SEARCH failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(message)))
	}
}

func buildSearchRequestBody(scopePath string) ([]byte, error) {
	var href bytes.Buffer
	if err := xml.EscapeText(&href, []byte(scopePath)); err != nil {
		return nil, err
	}
	request := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<d:searchrequest xmlns:d="DAV:" xmlns:oc="http://owncloud.org/ns">
  <d:basicsearch>
    <d:select>
      <d:prop>
        <d:displayname/>
        <d:getcontenttype/>
        <d:getetag/>
        <d:getcontentlength/>
        <d:getlastmodified/>
        <d:resourcetype/>
        <oc:fileid/>
        <oc:checksums/>
      </d:prop>
    </d:select>
    <d:from>
      <d:scope>
        <d:href>%s</d:href>
        <d:depth>infinity</d:depth>
      </d:scope>
    </d:from>
    <d:where>
      <d:not>
        <d:is-collection/>
      </d:not>
    </d:where>
    <d:orderby/>
  </d:basicsearch>
</d:searchrequest>`, href.String())
	return []byte(request), nil
}

func decodeSearchResponse(body io.Reader, davRootPath string) ([]SearchEntry, error) {
	var multiStatus searchMultiStatus
	if err := xml.NewDecoder(body).Decode(&multiStatus); err != nil {
		return nil, err
	}

	entries := make([]SearchEntry, 0, len(multiStatus.Responses))
	for _, response := range multiStatus.Responses {
		prop, ok := successfulSearchProp(response.PropStats)
		if !ok || prop.ResourceType.Collection != nil {
			continue
		}

		decodedPath, err := decodeSearchHref(response.Href, davRootPath)
		if err != nil {
			return nil, err
		}
		entries = append(entries, SearchEntry{
			Path: decodedPath,
			Info: searchFileInfo{
				name:    pathpkg.Base(decodedPath),
				size:    parseSearchSize(prop.ContentLength),
				modTime: parseSearchTime(prop.LastModified),
			},
			ContentType: prop.ContentType,
			ETag:        prop.ETag,
			FileID:      prop.FileID,
			Checksums:   append([]string(nil), prop.Checksums...),
		})
	}
	return entries, nil
}

func successfulSearchProp(propStats []searchPropStat) (searchProp, bool) {
	for _, propStat := range propStats {
		if strings.Contains(propStat.Status, " 200 ") {
			return propStat.Prop, true
		}
	}
	return searchProp{}, false
}

func decodeSearchHref(href string, davRootPath string) (string, error) {
	href = strings.TrimSpace(href)
	if href == "" {
		return "", errors.New("Nextcloud DAV SEARCH response did not include an href")
	}

	decodedPath := href
	if parsed, err := url.Parse(href); err == nil && parsed.Path != "" {
		decodedPath = parsed.Path
	}
	decodedPath, err := url.PathUnescape(decodedPath)
	if err != nil {
		return "", err
	}
	decodedPath = pathpkg.Clean(decodedPath)

	if davRootPath != "" && strings.HasPrefix(decodedPath, davRootPath) {
		decodedPath = strings.TrimPrefix(decodedPath, davRootPath)
		if decodedPath == "" {
			decodedPath = "/"
		}
	}
	if !strings.HasPrefix(decodedPath, "/") {
		decodedPath = "/" + decodedPath
	}
	return pathpkg.Clean(decodedPath), nil
}

func parseSearchSize(raw string) int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	size, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0
	}
	return size
}

func parseSearchTime(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	t, err := http.ParseTime(raw)
	if err != nil {
		return time.Time{}
	}
	return t
}
