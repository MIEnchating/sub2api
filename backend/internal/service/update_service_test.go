//go:build unit

package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

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

func (s *updateServiceGitHubClientStub) FetchRecentReleases(context.Context, string, int) ([]*GitHubRelease, error) {
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

func (s *updateServiceGitHubClientStub) FetchChecksumFile(context.Context, string) ([]byte, error) {
	panic("FetchChecksumFile should not be called when no update is available")
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

func TestUpdateServiceSourceBuildTracksOwnVersionFile(t *testing.T) {
	client := &updateServiceGitHubClientStub{repositoryFile: []byte("0.1.176-overdraft.2\n")}
	svc := NewUpdateService(
		&updateServiceCacheStub{},
		client,
		"0.1.176-overdraft.1",
		"source",
	)

	info, err := svc.CheckUpdate(context.Background(), true)

	require.NoError(t, err)
	require.Equal(t, "0.1.176-overdraft.1", info.CurrentVersion)
	require.Equal(t, "0.1.176-overdraft.2", info.LatestVersion)
	require.True(t, info.HasUpdate)
	require.Equal(t, "source", info.BuildType)
	require.Equal(t, releaseSourceUpdateURL, info.ReleaseInfo.HTMLURL)
	require.Contains(t, client.latestRepos, officialUpstreamRepo)
	require.NotContains(t, client.latestRepos, releaseRepo, "源码构建不能查询自身的二进制 Release")
}

func TestUpdateServiceSourceBuildIgnoresLegacyOfficialCache(t *testing.T) {
	cache := &updateServiceCacheStub{data: `{"latest":"0.1.176","timestamp":4102444800}`}
	client := &updateServiceGitHubClientStub{repositoryFile: []byte("0.1.176-overdraft.1\n")}
	svc := NewUpdateService(cache, client, "0.1.176-overdraft.1", "source")

	info, err := svc.CheckUpdate(context.Background(), false)

	require.NoError(t, err)
	require.False(t, info.HasUpdate)
	require.Equal(t, "0.1.176-overdraft.1", info.LatestVersion)
	require.Equal(t, 2, client.fileCalls)
}

func TestUpdateServiceSourceBuildRejectsBinaryUpdate(t *testing.T) {
	svc := NewUpdateService(
		&updateServiceCacheStub{},
		&updateServiceGitHubClientStub{},
		"0.1.176-overdraft.1",
		"source",
	)

	err := svc.PerformUpdate(context.Background())

	require.ErrorIs(t, err, ErrSourceBuildUpdateRequired)
}

func TestUpdateServiceReleaseBuildUsesOwnRepository(t *testing.T) {
	client := &updateServiceGitHubClientStub{release: &GitHubRelease{TagName: "v0.1.177-overdraft.1"}}
	svc := NewUpdateService(&updateServiceCacheStub{}, client, "0.1.176-overdraft.1", "release")

	info, err := svc.CheckUpdate(context.Background(), true)

	require.NoError(t, err)
	require.True(t, info.HasUpdate)
	require.Contains(t, client.latestRepos, releaseRepo)
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
			overdraftUpstreamRepo + "\x00" + overdraftUpstreamBranch + "\x00" + overdraftUpstreamVersion: []byte("0.1.179-overdraft.2\n"),
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
			ID:             "overdraft",
			Repository:     overdraftUpstreamRepo,
			Version:        "0.1.179-overdraft.2",
			HTMLURL:        overdraftUpstreamUpdateURL,
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
			baseRepo: overdraftUpstreamRepo,
			baseRef:  overdraftUpstreamBaseline,
			headRepo: overdraftUpstreamRepo,
			headRef:  overdraftUpstreamBranch,
		},
	}, client.comparisonCalls)
}

func TestCompareVersionsSupportsForkPrereleaseRevisions(t *testing.T) {
	require.Less(t, compareVersions("0.1.176-overdraft.1", "0.1.176-overdraft.2"), 0)
	require.Zero(t, compareVersions("v0.1.176-overdraft.2", "0.1.176-overdraft.2"))
	require.Greater(t, compareVersions("0.1.177-overdraft.1", "0.1.176-overdraft.9"), 0)
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
