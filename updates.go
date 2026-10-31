package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const latestReleaseEndpoint = "https://api.github.com/repos/lbk-ones/wd-reader/releases/latest"
const releasePage = "https://github.com/lbk-ones/wd-reader/releases"

type UpdateInfo struct {
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	Available      bool   `json:"available"`
	Comparable     bool   `json:"comparable"`
	URL            string `json:"url"`
}

var releaseVersionPattern = regexp.MustCompile(`^[vV]?([0-9]+)\.([0-9]+)(?:\.([0-9]+))?(?:-([0-9A-Za-z.-]+))?(?:\+[0-9A-Za-z.-]+)?$`)

func releaseVersion(value string) ([3]uint64, bool, bool) {
	var parts [3]uint64
	m := releaseVersionPattern.FindStringSubmatch(strings.TrimSpace(value))
	if m == nil {
		return parts, false, false
	}
	for i := 0; i < 3; i++ {
		if m[i+1] == "" {
			continue
		}
		n, err := strconv.ParseUint(m[i+1], 10, 64)
		if err != nil {
			return parts, false, false
		}
		parts[i] = n
	}
	return parts, m[4] != "", true
}

func fetchUpdate(client *http.Client, endpoint, current string) (UpdateInfo, error) {
	info := UpdateInfo{CurrentVersion: current, URL: releasePage}
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return info, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "wd-reader-update-check")
	res, err := client.Do(req)
	if err != nil {
		return info, fmt.Errorf("版本检查连接失败: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return info, fmt.Errorf("版本检查失败 (HTTP %d)", res.StatusCode)
	}
	var release struct {
		Tag        string `json:"tag_name"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
	}
	if err = json.NewDecoder(io.LimitReader(res.Body, 2<<20)).Decode(&release); err != nil {
		return info, fmt.Errorf("无法解析版本信息: %w", err)
	}
	latest, latestPre, ok := releaseVersion(release.Tag)
	if !ok || latestPre || release.Draft || release.Prerelease {
		return info, fmt.Errorf("未获取到有效的正式版本")
	}
	info.LatestVersion = release.Tag
	installed, pre, ok := releaseVersion(current)
	info.Comparable = ok
	if ok {
		for i := range latest {
			if latest[i] != installed[i] {
				info.Available = latest[i] > installed[i]
				return info, nil
			}
		}
		info.Available = pre
	}
	return info, nil
}

var updateCache struct {
	sync.Mutex
	info      UpdateInfo
	checkedAt time.Time
}

func (a *App) CheckForUpdates(force bool) (UpdateInfo, error) {
	updateCache.Lock()
	defer updateCache.Unlock()
	if !force && updateCache.info.CurrentVersion == version && time.Since(updateCache.checkedAt) < time.Hour {
		return updateCache.info, nil
	}
	info, err := fetchUpdate(&http.Client{Timeout: 8 * time.Second}, latestReleaseEndpoint, version)
	if err == nil {
		updateCache.info = info
		updateCache.checkedAt = time.Now()
	}
	return info, err
}
