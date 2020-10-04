package flint

import (
	"fmt"
	"github.com/google/go-github/github"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func TestGitHubFetcherRequiresClientForFetchRepository(t *testing.T) {
	fetcher := &GitHubFetcher{}
	repo, err := fetcher.FetchRepository("tyhal/flint")
	assert.Nil(t, repo)
	assert.NotNil(t, err)
}

func TestGitHubFetcher_FetchRepository(t *testing.T) {
	setupGitHubFetcherTest()
	defer tearDownGitHubFetcherTest()

	mux.HandleFunc("/repos/octokit/octokit.rb", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		respondWithJSON(w, loadFixture("octokit.rb.json"))
	})

	fetcher := &GitHubFetcher{&*client}
	repo, err := fetcher.FetchRepository("octokit/octokit.rb")
	if assert.NotNil(t, repo) {
		assert.Equal(t, "Ruby toolkit for the GitHub API", repo.Description)
		assert.Equal(t, "http://octokit.github.io/octokit.rb/", repo.Homepage)
	}
	assert.Nil(t, err)
}

func TestGitHubFetcherHandlesAPIErrorsForFetchRepository(t *testing.T) {
	setupGitHubFetcherTest()
	defer tearDownGitHubFetcherTest()

	mux.HandleFunc("/repos/octokit/octokit.rb", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		respondWithStatus(w, 404)
	})

	fetcher := &GitHubFetcher{&*client}
	repo, err := fetcher.FetchRepository("octokit/octokit.rb")
	assert.Nil(t, repo)
	assert.NotNil(t, err)
}

func TestGitHubFetcherHandlesNameParsingErrorForFetchRepository(t *testing.T) {
	fetcher := NewGitHubFetcherWithToken("foo")
	repo, err := fetcher.FetchRepository("foo")
	assert.Nil(t, repo)
	if assert.NotNil(t, err) {
		assert.Equal(t, "Invalid GitHub repository: foo", err.Error())
	}
}

func TestGitHubFetcherRequiresClientForFetchTree(t *testing.T) {
	fetcher := &GitHubFetcher{}
	paths, err := fetcher.FetchTree("tyhal/flint")
	assert.Nil(t, paths)
	assert.NotNil(t, err)
}

func TestGitHubFetcherHandlesAPIErrorsForFetchTree(t *testing.T) {
	setupGitHubFetcherTest()
	defer tearDownGitHubFetcherTest()

	mux.HandleFunc("/repos/octokit/octokit.rb/git/trees/master", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		respondWithStatus(w, 404)
	})

	fetcher := &GitHubFetcher{&*client}
	repo, err := fetcher.FetchTree("octokit/octokit.rb")
	assert.Nil(t, repo)
	assert.NotNil(t, err)
}

func TestGitHubFetcherHandlesNameParsingErrorForFetchTree(t *testing.T) {
	fetcher := NewGitHubFetcherWithToken("foo")
	repo, err := fetcher.FetchTree("foo")
	assert.Nil(t, repo)
	if assert.NotNil(t, err) {
		assert.Equal(t, "Invalid GitHub repository: foo", err.Error())
	}
}

func TestGitHubFetcher_FetchTree(t *testing.T) {
	setupGitHubFetcherTest()
	defer tearDownGitHubFetcherTest()

	mux.HandleFunc("/repos/octokit/octokit.rb/git/trees/master", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		respondWithJSON(w, loadFixture("octokit.rb_tree.json"))
	})

	fetcher := &GitHubFetcher{&*client}
	paths, err := fetcher.FetchTree("octokit/octokit.rb")
	assert.Contains(t, paths, "Rakefile")
	assert.Contains(t, paths, "README.md")
	assert.Nil(t, err)
}

func TestGitHubFetcher_FetchReleases(t *testing.T) {
	setupGitHubFetcherTest()
	defer tearDownGitHubFetcherTest()

	mux.HandleFunc("/repos/octokit/octokit.rb/releases", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		respondWithJSON(w, loadFixture("octokit.rb_releases.json"))
	})

	fetcher := &GitHubFetcher{&*client}
	paths, err := fetcher.FetchReleases("octokit/octokit.rb")
	assert.Contains(t, paths, "v3.2.0")
	assert.Contains(t, paths, "v2.0.0.pre")
	assert.NotContains(t, paths, "") // ignore tag-only releases
	assert.Nil(t, err)
}

func TestGitHubFetcher_ParseFullName(t *testing.T) {
	fetcher := &GitHubFetcher{}

	owner, name, err := fetcher.ParseFullName("tyhal/flint")
	assert.Equal(t, "tyhal", owner)
	assert.Equal(t, "flint", name)
	assert.Nil(t, err)

	owner, name, err = fetcher.ParseFullName("tyhalflint")
	assert.Empty(t, owner)
	assert.Empty(t, name)

	if assert.NotNil(t, err) {
		assert.Equal(t, "Invalid GitHub repository: tyhalflint", err.Error())
	}
}

var (
	mux    *http.ServeMux
	client *github.Client
	server *httptest.Server
)

const (
	gitHubAPIURL = "https://api.github.com"
	userAgent    = "Octokit Go "
)

type TestTransport struct {
	http.RoundTripper
	overrideURL *url.URL
}

func (t TestTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = cloneRequest(req)
	req.Header.Set("X-Original-Scheme", req.URL.Scheme)
	req.URL.Scheme = t.overrideURL.Scheme
	req.URL.Host = t.overrideURL.Host
	return t.RoundTripper.RoundTrip(req)
}

func cloneRequest(r *http.Request) *http.Request {
	r2 := new(http.Request)
	*r2 = *r
	r2.URL, _ = url.Parse(r.URL.String())
	r2.Header = make(http.Header)
	for k, s := range r.Header {
		r2.Header[k] = s
	}
	return r2
}

func setupGitHubFetcherTest() {
	// test server
	mux = http.NewServeMux()
	server = httptest.NewServer(mux)

	client = github.NewClient(nil)
	url, _ := url.Parse(server.URL)
	client.BaseURL = url
}

// teardown closes the test HTTP server.
func tearDownGitHubFetcherTest() {
	server.Close()
}

func respondWithJSON(w http.ResponseWriter, s string) {
	header := w.Header()
	header.Set("Content-Type", "application/json")
	respondWith(w, s)
}

func respondWithStatus(w http.ResponseWriter, statusCode int) {
	w.WriteHeader(statusCode)
}

func respondWith(w http.ResponseWriter, s string) {
	fmt.Fprint(w, s)
}

func testMethod(t *testing.T, r *http.Request, want string) {
	assert.Equal(t, want, r.Method)
}

func loadFixture(f string) string {
	pwd, _ := os.Getwd()
	p := filepath.Join(pwd, "..", "fixtures", f)
	c, _ := ioutil.ReadFile(p)
	return string(c)
}
