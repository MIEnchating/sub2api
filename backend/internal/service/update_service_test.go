//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/updatepolicy"
	"github.com/stretchr/testify/require"
)

type updateServiceCacheStub struct {
	data string
}

func (s *updateServiceCacheStub) GetUpdateInfo(context.Context) (string, error) {
	if s.data == "" {
		return "", errors.New("cache miss")
	}
	return s.data, nil
}

func (s *updateServiceCacheStub) SetUpdateInfo(_ context.Context, data string, _ time.Duration) error {
	s.data = data
	return nil
}

type updateServiceGitHubClientStub struct {
	mu                sync.Mutex
	manifests         map[string][]byte
	recentRepo        string
	release           *GitHubRelease
	releases          map[string]*GitHubRelease
	recentReleases    []*GitHubRelease
	recentErr         error
	repositoryFile    []byte
	repositoryFiles   map[string][]byte
	repositoryFileErr error
	latestRepo        string
	latestRepos       []string
	fileRepo          string
	fileRef           string
	filePath          string
	fileCalls         int
	comparisonCalls   []updateServiceComparisonCall
}

type updateServiceComparisonCall struct {
	baseRepo string
	baseRef  string
	headRepo string
	headRef  string
}

func (s *updateServiceGitHubClientStub) FetchLatestRelease(_ context.Context, repo string) (*GitHubRelease, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.latestRepo = repo
	s.latestRepos = append(s.latestRepos, repo)
	if release, ok := s.releases[repo]; ok {
		return release, nil
	}
	return s.release, nil
}

func (s *updateServiceGitHubClientStub) FetchRecentReleases(_ context.Context, repo string, _ int) ([]*GitHubRelease, error) {
	s.recentRepo = repo
	return s.recentReleases, s.recentErr
}

func (s *updateServiceGitHubClientStub) FetchRepositoryFile(_ context.Context, repo, ref, filePath string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fileCalls++
	s.fileRepo = repo
	s.fileRef = ref
	s.filePath = filePath
	if value, ok := s.repositoryFiles[repo+"\x00"+ref+"\x00"+filePath]; ok {
		return value, nil
	}
	return s.repositoryFile, s.repositoryFileErr
}

func (s *updateServiceGitHubClientStub) FetchComparison(_ context.Context, baseRepo, baseRef, headRepo, headRef string) (*GitHubComparison, error) {
	s.mu.Lock()
	s.comparisonCalls = append(s.comparisonCalls, updateServiceComparisonCall{
		baseRepo: baseRepo,
		baseRef:  baseRef,
		headRepo: headRepo,
		headRef:  headRef,
	})
	s.mu.Unlock()
	return &GitHubComparison{Status: "identical", HTMLURL: "https://github.com/compare"}, nil
}

func (s *updateServiceGitHubClientStub) DownloadFile(context.Context, string, string, int64) error {
	panic("DownloadFile should not be called when no update is available")
}

func (s *updateServiceGitHubClientStub) FetchChecksumFile(_ context.Context, url string) ([]byte, error) {
	data, ok := s.manifests[url]
	if !ok {
		return nil, errors.New("manifest missing")
	}
	return data, nil
}

func TestUpdateServicePerformUpdateNoUpdateReturnsSentinel(t *testing.T) {
	svc := NewUpdateService(
		&updateServiceCacheStub{},
		&updateServiceGitHubClientStub{
			release: &GitHubRelease{
				TagName: "v0.1.132",
				Name:    "v0.1.132",
			},
		},
		"0.1.132",
		"release",
	)

	err := svc.PerformUpdate(context.Background())

	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNoUpdateAvailable))
	require.ErrorIs(t, err, ErrNoUpdateAvailable)
}

func publishedFixture(version string) (*GitHubRelease, []byte) {
	url, _ := updatepolicy.AssetURL(version)
	raw, _ := json.Marshal(updatepolicy.Manifest{SchemaVersion: 1, Version: version, Revision: strings.Repeat("a", 40), Images: map[string]string{runtime.GOARCH: updatepolicy.ImageRepository + "@sha256:" + strings.Repeat("b", 64)}})
	return &GitHubRelease{TagName: "v" + version, Prerelease: false, HTMLURL: "https://github.com/" + releaseRepo + "/releases/tag/v" + version, Assets: []GitHubAsset{{Name: updatepolicy.AssetName, BrowserDownloadURL: url, Size: int64(len(raw))}}}, raw
}
func TestUpdateServiceAdvertisesOnlyReadyPublishedReleases(t *testing.T) {
	ready, raw := publishedFixture("2026.10.20")
	client := &updateServiceGitHubClientStub{recentReleases: []*GitHubRelease{{TagName: "v2026.10.21"}, ready}, manifests: map[string][]byte{ready.Assets[0].BrowserDownloadURL: raw}}
	for _, build := range []string{"source", "release"} {
		svc := NewUpdateService(&updateServiceCacheStub{}, client, "2026.10.13", build)
		info, err := svc.CheckUpdate(context.Background(), true)
		require.NoError(t, err)
		require.True(t, info.HasUpdate)
		require.Equal(t, "2026.10.20", info.LatestVersion)
		require.Equal(t, ready.HTMLURL, info.ReleaseInfo.HTMLURL)
		require.Equal(t, customUpstreamRepo, client.fileRepo)
		require.Equal(t, releaseRepo, client.recentRepo)
	}
}

func TestUpdateServiceWithoutCache(t *testing.T) {
	for _, force := range []bool{false, true} {
		for _, unavailable := range []bool{false, true} {
			t.Run(fmt.Sprintf("force=%t/unavailable=%t", force, unavailable), func(t *testing.T) {
				ready, raw := publishedFixture("2026.10.20")
				client := &updateServiceGitHubClientStub{recentReleases: []*GitHubRelease{ready}, manifests: map[string][]byte{ready.Assets[0].BrowserDownloadURL: raw}}
				if unavailable {
					client.recentErr = errors.New("release service unavailable")
				}
				svc := NewUpdateService(nil, client, "2026.10.13", "release")
				info, err := svc.CheckUpdate(context.Background(), force)
				require.NoError(t, err)
				require.False(t, info.Cached)
				require.Equal(t, !unavailable, info.HasUpdate)
				require.Len(t, info.Upstreams, 2)
				if unavailable {
					require.Contains(t, info.Warning, "release service unavailable")
				} else {
					require.Equal(t, "2026.10.20", info.LatestVersion)
				}
			})
		}
	}
}
func TestUpdateServiceIgnoresLegacyBranchCache(t *testing.T) {
	cache := &updateServiceCacheStub{data: `{"latest":"2026.10.25","timestamp":4102444800,"repository":"MIEnchating/sub2api","build_type":"release"}`}
	client := &updateServiceGitHubClientStub{repositoryFile: []byte("v0.2.17-custom.99")}
	svc := NewUpdateService(cache, client, "2026.10.13", "release")
	info, err := svc.CheckUpdate(context.Background(), false)
	require.NoError(t, err)
	require.False(t, info.HasUpdate)
	require.Equal(t, "2026.10.13", info.LatestVersion)
	require.Equal(t, customUpstreamRepo, client.fileRepo)
	require.Equal(t, releaseRepo, client.recentRepo)
}
func TestUpdateServiceManifestMismatchDoesNotAdvertiseUpdate(t *testing.T) {
	ready, raw := publishedFixture("2026.10.20")
	client := &updateServiceGitHubClientStub{recentReleases: []*GitHubRelease{ready}, manifests: map[string][]byte{ready.Assets[0].BrowserDownloadURL: []byte(strings.ReplaceAll(string(raw), "2026.10.20", "2026.10.21"))}}
	svc := NewUpdateService(&updateServiceCacheStub{}, client, "2026.10.13", "release")
	info, err := svc.CheckUpdate(context.Background(), true)
	require.NoError(t, err)
	require.False(t, info.HasUpdate)
	require.Contains(t, info.Warning, "mismatch")
}
func TestUpdateServiceSourceBuildRejectsBinaryUpdate(t *testing.T) {
	svc := NewUpdateService(&updateServiceCacheStub{}, &updateServiceGitHubClientStub{}, "2026.10.13", "source")
	require.ErrorIs(t, svc.PerformUpdate(context.Background()), ErrSourceBuildUpdateRequired)
}

type offlineUpdateAgent struct{}

func (offlineUpdateAgent) Health(context.Context) error { return errors.New("offline") }
func (offlineUpdateAgent) Start(context.Context, string, string) (SourceUpdateJob, error) {
	return SourceUpdateJob{}, ErrSourceUpdaterUnavailable
}
func (offlineUpdateAgent) Status(context.Context, string) (SourceUpdateJob, error) {
	return SourceUpdateJob{}, ErrSourceUpdaterUnavailable
}
func TestManagedReleaseNeverFallsBackToBinaryMutation(t *testing.T) {
	svc := NewUpdateService(&updateServiceCacheStub{}, &updateServiceGitHubClientStub{}, "2026.10.13", "release")
	svc.SetSourceUpdateAgent(offlineUpdateAgent{})
	info, err := svc.CheckUpdate(context.Background(), true)
	require.NoError(t, err)
	require.True(t, info.HostUpdateManaged)
	require.False(t, info.SourceUpdateEnabled)
	require.ErrorIs(t, svc.PerformUpdate(context.Background()), ErrSourceUpdaterUnavailable)
	require.Error(t, svc.Rollback())
	require.Error(t, svc.RollbackToVersion(context.Background(), "0.2.17-custom.11"))
}

func TestUpdateServiceReportsBothUpstreamVersions(t *testing.T) {
	client := &updateServiceGitHubClientStub{
		releases: map[string]*GitHubRelease{
			releaseRepo: {
				TagName: "v2026.8.22",
				HTMLURL: "https://github.com/MIEnchating/sub2api/releases/tag/v2026.8.22",
			},
			officialUpstreamRepo: {
				TagName: "v0.1.179",
				HTMLURL: "https://github.com/Wei-Shaw/sub2api/releases/tag/v0.1.179",
			},
		},
		repositoryFiles: map[string][]byte{
			customUpstreamRepo + "\x00" + customUpstreamBranch + "\x00" + customUpstreamVersion: []byte("0.2.4-custom.1\n"),
		},
	}
	svc := NewUpdateService(&updateServiceCacheStub{}, client, "2026.8.22", "release")

	info, err := svc.CheckUpdate(context.Background(), true)

	require.NoError(t, err)
	require.Equal(t, "2026.8.22", info.CurrentVersion)
	require.False(t, info.HasUpdate)
	require.Equal(t, []UpstreamVersionInfo{
		{
			ID:             "official",
			Repository:     officialUpstreamRepo,
			Version:        "0.1.179",
			HTMLURL:        "https://github.com/Wei-Shaw/sub2api/releases/tag/v0.1.179",
			CompareURL:     "https://github.com/compare",
			CompareChecked: true,
		},
		{
			ID:             "custom",
			Repository:     customUpstreamRepo,
			Version:        "0.2.4-custom.1",
			HTMLURL:        customUpstreamUpdateURL,
			CompareURL:     "https://github.com/compare",
			CompareChecked: true,
		},
	}, info.Upstreams)
	require.ElementsMatch(t, []updateServiceComparisonCall{
		{
			baseRepo: officialUpstreamRepo,
			baseRef:  officialUpstreamBaseline,
			headRepo: officialUpstreamRepo,
			headRef:  "main",
		},
		{
			baseRepo: customUpstreamRepo,
			baseRef:  customUpstreamBaseline,
			headRepo: customUpstreamRepo,
			headRef:  customUpstreamBranch,
		},
	}, client.comparisonCalls)
}

func TestCompareVersionsSupportsForkPrereleaseRevisions(t *testing.T) {
	require.Less(t, compareVersions("0.1.176-overdraft.1", "0.1.176-overdraft.2"), 0)
	require.Zero(t, compareVersions("v0.1.176-overdraft.2", "0.1.176-overdraft.2"))
	require.Greater(t, compareVersions("0.1.177-overdraft.1", "0.1.176-overdraft.9"), 0)
	require.Less(t, compareVersions("0.1.183-overdraft.8", "0.1.184-custom.1"), 0)
	require.Less(t, compareVersions("0.1.184-custom.1", "0.1.184-custom.2"), 0)
}

func newRollbackTestService(current string, releases []*GitHubRelease) *UpdateService {
	return NewUpdateService(
		&updateServiceCacheStub{},
		&updateServiceGitHubClientStub{recentReleases: releases},
		current,
		"release",
	)
}

func TestUpdateServiceListRollbackVersionsFiltersAndCaps(t *testing.T) {
	releases := []*GitHubRelease{
		{TagName: "v0.1.148", PublishedAt: "2026-07-09T00:00:00Z"},                       // newer than current: excluded
		{TagName: "v0.1.147", PublishedAt: "2026-07-08T00:00:00Z"},                       // current: excluded
		{TagName: "v0.1.146-rc1", PublishedAt: "2026-07-07T12:00:00Z", Prerelease: true}, // prerelease: excluded
		{TagName: "v0.1.146", PublishedAt: "2026-07-07T00:00:00Z"},
		{TagName: "v0.1.145", PublishedAt: "2026-07-06T00:00:00Z", Draft: true}, // draft: excluded
		{TagName: "v0.1.144", PublishedAt: "2026-07-05T00:00:00Z"},
		{TagName: "v0.1.144", PublishedAt: "2026-07-05T00:00:00Z"}, // duplicate: excluded
		{TagName: "v0.1.143", PublishedAt: "2026-07-04T00:00:00Z"},
		{TagName: "v0.1.142", PublishedAt: "2026-07-03T00:00:00Z"}, // beyond cap of 3: excluded
	}
	svc := newRollbackTestService("0.1.147", releases)

	versions, err := svc.ListRollbackVersions(context.Background())

	require.NoError(t, err)
	require.Len(t, versions, 3)
	require.Equal(t, "0.1.146", versions[0].Version)
	require.Equal(t, "0.1.144", versions[1].Version)
	require.Equal(t, "0.1.143", versions[2].Version)
}

func TestUpdateServiceListRollbackVersionsSortsUnorderedInput(t *testing.T) {
	releases := []*GitHubRelease{
		{TagName: "v0.1.144"},
		{TagName: "v0.1.146"},
		{TagName: "v0.1.145"},
	}
	svc := newRollbackTestService("0.1.147", releases)

	versions, err := svc.ListRollbackVersions(context.Background())

	require.NoError(t, err)
	require.Len(t, versions, 3)
	require.Equal(t, "0.1.146", versions[0].Version)
	require.Equal(t, "0.1.145", versions[1].Version)
	require.Equal(t, "0.1.144", versions[2].Version)
}

func TestUpdateServiceListRollbackVersionsEmptyWhenNoneOlder(t *testing.T) {
	releases := []*GitHubRelease{
		{TagName: "v0.1.147"},
		{TagName: "v0.1.148"},
	}
	svc := newRollbackTestService("0.1.147", releases)

	versions, err := svc.ListRollbackVersions(context.Background())

	require.NoError(t, err)
	require.Empty(t, versions)
}

func TestUpdateServiceListRollbackVersionsPropagatesFetchError(t *testing.T) {
	svc := NewUpdateService(
		&updateServiceCacheStub{},
		&updateServiceGitHubClientStub{recentErr: errors.New("github unavailable")},
		"0.1.147",
		"release",
	)

	_, err := svc.ListRollbackVersions(context.Background())

	require.Error(t, err)
	require.Contains(t, err.Error(), "github unavailable")
}

func TestUpdateServiceRollbackToVersionRejectsDisallowedTargets(t *testing.T) {
	releases := []*GitHubRelease{
		{TagName: "v0.1.148"},
		{TagName: "v0.1.147"},
		{TagName: "v0.1.146"},
		{TagName: "v0.1.145"},
		{TagName: "v0.1.144"},
		{TagName: "v0.1.143"},
		{TagName: "v0.1.142"},
	}
	svc := newRollbackTestService("0.1.147", releases)

	for _, target := range []string{
		"",         // empty
		"0.1.147",  // current version
		"v0.1.147", // current version with prefix
		"0.1.148",  // newer than current
		"0.1.142",  // older than the 3 most recent
		"9.9.9",    // nonexistent
	} {
		err := svc.RollbackToVersion(context.Background(), target)
		require.ErrorIs(t, err, ErrRollbackVersionNotAllowed, "target %q should be rejected", target)
	}
}

func TestUpdateServiceRollbackToVersionAcceptsVPrefix(t *testing.T) {
	// No platform asset in the release: the target passes the allowlist check
	// and fails later at asset lookup, proving the version itself was accepted.
	releases := []*GitHubRelease{
		{TagName: "v0.1.147"},
		{TagName: "v0.1.146"},
	}
	svc := newRollbackTestService("0.1.147", releases)

	err := svc.RollbackToVersion(context.Background(), "v0.1.146")

	require.Error(t, err)
	require.NotErrorIs(t, err, ErrRollbackVersionNotAllowed)
	require.Contains(t, err.Error(), "no compatible release found")
}

func TestUpdateServiceRejectsUnpublishedOrForeignReleases(t *testing.T) {
	for _, kind := range []string{"draft", "prerelease", "foreign_manifest", "foreign_image", "unsupported_arch"} {
		t.Run(kind, func(t *testing.T) {
			release, raw := publishedFixture("2026.10.20")
			url := release.Assets[0].BrowserDownloadURL
			switch kind {
			case "draft":
				release.Draft = true
			case "prerelease":
				release.Prerelease = true
			case "foreign_manifest":
				release.Assets[0].BrowserDownloadURL = strings.ReplaceAll(url, releaseRepo, customUpstreamRepo)
			case "foreign_image":
				raw = []byte(strings.ReplaceAll(string(raw), updatepolicy.ImageRepository, "ghcr.io/deanzfc/sub2api"))
			case "unsupported_arch":
				raw = []byte(strings.ReplaceAll(string(raw), runtime.GOARCH, "unsupported"))
			}
			client := &updateServiceGitHubClientStub{recentReleases: []*GitHubRelease{release}, manifests: map[string][]byte{url: raw}}
			svc := NewUpdateService(&updateServiceCacheStub{}, client, "2026.10.13", "release")
			info, err := svc.CheckUpdate(context.Background(), true)
			require.NoError(t, err)
			require.False(t, info.HasUpdate)
		})
	}
}

func TestUpdateServiceReadyCacheRetainsUpstreamsAndLiveHostState(t *testing.T) {
	release, raw := publishedFixture("2026.10.20")
	client := &updateServiceGitHubClientStub{release: &GitHubRelease{TagName: "v0.2.17"}, repositoryFile: []byte("v0.2.17-custom.18"), recentReleases: []*GitHubRelease{release}, manifests: map[string][]byte{release.Assets[0].BrowserDownloadURL: raw}}
	svc := NewUpdateService(&updateServiceCacheStub{}, client, "2026.10.13", "release")
	first, err := svc.CheckUpdate(context.Background(), true)
	require.NoError(t, err)
	require.True(t, first.HasUpdate)
	client.recentErr = errors.New("offline")
	svc.SetSourceUpdateAgent(offlineUpdateAgent{})
	cached, err := svc.CheckUpdate(context.Background(), false)
	require.NoError(t, err)
	require.True(t, cached.Cached)
	require.True(t, cached.HasUpdate)
	require.True(t, cached.HostUpdateManaged)
	require.False(t, cached.SourceUpdateEnabled)
	require.Equal(t, first.Upstreams, cached.Upstreams)
	versions, err := svc.ListRollbackVersions(context.Background())
	require.NoError(t, err)
	require.Empty(t, versions)
}
