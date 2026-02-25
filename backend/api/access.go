package api

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/goccy/go-yaml"
)

type AccessConfig struct {
	AllowedUsers  []string `yaml:"allowedUsers"`
	AllowedGroups []string `yaml:"allowedGroups"`
}

type openShiftGroupList struct {
	Items []openShiftGroup `json:"items"`
}

type openShiftGroup struct {
	Metadata struct {
		Name string `json:"name"`
	} `json:"metadata"`
	Users []string `json:"users"`
}

type AccessChecker struct {
	mu           sync.RWMutex
	config       AccessConfig
	path         string
	groupMembers map[string]map[string]bool // group name → set of usernames
	httpClient   *http.Client
	apiServer    string
	tokenPath    string
	caPath       string
}

func NewAccessChecker(path string) (*AccessChecker, error) {
	ac := &AccessChecker{
		path:         path,
		groupMembers: make(map[string]map[string]bool),
		apiServer:    "https://kubernetes.default.svc",
		tokenPath:    "/var/run/secrets/kubernetes.io/serviceaccount/token",
		caPath:       "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt",
	}

	ac.httpClient = ac.buildHTTPClient()

	if err := ac.load(); err != nil {
		return nil, err
	}
	ac.refreshGroups()
	go ac.watch()
	go ac.refreshGroupsLoop()
	return ac, nil
}

func (ac *AccessChecker) buildHTTPClient() *http.Client {
	tlsConfig := &tls.Config{}
	if caCert, err := os.ReadFile(ac.caPath); err == nil {
		pool := x509.NewCertPool()
		pool.AppendCertsFromPEM(caCert)
		tlsConfig.RootCAs = pool
	}
	return &http.Client{
		Timeout:   10 * time.Second,
		Transport: &http.Transport{TLSClientConfig: tlsConfig},
	}
}

func (ac *AccessChecker) load() error {
	data, err := os.ReadFile(ac.path)
	if err != nil {
		return err
	}
	var config AccessConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return err
	}
	ac.mu.Lock()
	ac.config = config
	ac.mu.Unlock()
	log.Printf("Loaded access config: %d users, %d groups", len(config.AllowedUsers), len(config.AllowedGroups))
	return nil
}

func (ac *AccessChecker) watch() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("Failed to create file watcher: %v", err)
		return
	}
	defer watcher.Close()

	dir := filepath.Dir(ac.path)
	if err := watcher.Add(dir); err != nil {
		log.Printf("Failed to watch %s: %v", dir, err)
		return
	}

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
				if err := ac.load(); err != nil {
					log.Printf("Failed to reload access config: %v", err)
				}
				ac.refreshGroups()
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Printf("File watcher error: %v", err)
		}
	}
}

// refreshGroupsLoop periodically fetches group membership from the OpenShift API.
func (ac *AccessChecker) refreshGroupsLoop() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		ac.refreshGroups()
	}
}

func (ac *AccessChecker) refreshGroups() {
	token, err := os.ReadFile(ac.tokenPath)
	if err != nil {
		log.Printf("Skipping group refresh (no SA token): %v", err)
		return
	}

	ac.mu.RLock()
	allowedGroups := make([]string, len(ac.config.AllowedGroups))
	copy(allowedGroups, ac.config.AllowedGroups)
	ac.mu.RUnlock()

	if len(allowedGroups) == 0 {
		return
	}

	url := fmt.Sprintf("%s/apis/user.openshift.io/v1/groups", ac.apiServer)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Printf("Failed to create groups request: %v", err)
		return
	}
	req.Header.Set("Authorization", "Bearer "+string(token))

	resp, err := ac.httpClient.Do(req)
	if err != nil {
		log.Printf("Failed to fetch groups: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Groups API returned %d: %s", resp.StatusCode, string(body))
		return
	}

	var groupList openShiftGroupList
	if err := json.NewDecoder(resp.Body).Decode(&groupList); err != nil {
		log.Printf("Failed to decode groups response: %v", err)
		return
	}

	allowedSet := make(map[string]bool, len(allowedGroups))
	for _, g := range allowedGroups {
		allowedSet[g] = true
	}

	newMembers := make(map[string]map[string]bool)
	for _, g := range groupList.Items {
		if !allowedSet[g.Metadata.Name] {
			continue
		}
		members := make(map[string]bool, len(g.Users))
		for _, u := range g.Users {
			members[u] = true
		}
		newMembers[g.Metadata.Name] = members
	}

	ac.mu.Lock()
	ac.groupMembers = newMembers
	ac.mu.Unlock()

	totalMembers := 0
	for _, m := range newMembers {
		totalMembers += len(m)
	}
	log.Printf("Refreshed group membership: %d groups, %d total members", len(newMembers), totalMembers)
}

// IsAllowed checks if a user is authorized. It checks the username and email
// against both the explicit allowed-users list and the OpenShift group membership.
// The email is needed because the oauth-proxy may send a short username (e.g. "ocohen")
// while OpenShift groups store the full identity (e.g. "ocohen@redhat.com").
func (ac *AccessChecker) IsAllowed(username, email string, headerGroups []string) bool {
	ac.mu.RLock()
	defer ac.mu.RUnlock()

	identities := []string{username}
	if email != "" && email != username {
		identities = append(identities, email)
	}

	for _, u := range ac.config.AllowedUsers {
		for _, id := range identities {
			if u == id {
				return true
			}
		}
	}

	// Check groups from X-Forwarded-Groups header
	for _, allowed := range ac.config.AllowedGroups {
		for _, g := range headerGroups {
			if allowed == g {
				return true
			}
		}
	}

	// Check groups resolved from the OpenShift Groups API
	for _, members := range ac.groupMembers {
		for _, id := range identities {
			if members[id] {
				return true
			}
		}
	}

	return false
}
