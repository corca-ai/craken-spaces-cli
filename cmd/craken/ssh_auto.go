package main

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	defaultManagedSSHIdentityName = "id_ed25519_spaces"
	defaultManagedKnownHostsName  = "spaces_known_hosts"
)

type sshIdentityMaterial struct {
	IdentityFile  string
	PublicKeyFile string
	PublicKey     string
}

type sshKnownHostsRecord struct {
	Host           string `json:"host"`
	Port           int    `json:"port"`
	PublicKey      string `json:"public_key"`
	KnownHostsLine string `json:"known_hosts_line"`
}

func ensureSSHIdentityMaterial(identityFile string) (sshIdentityMaterial, error) {
	if strings.TrimSpace(identityFile) != "" {
		material, err := loadSSHIdentityMaterial(identityFile)
		return material, err
	}
	defaultIdentityFile, err := defaultManagedSSHIdentityFile()
	if err != nil {
		return sshIdentityMaterial{}, err
	}
	if _, err := os.Stat(defaultIdentityFile); err == nil {
		material, loadErr := loadSSHIdentityMaterial(defaultIdentityFile)
		return material, loadErr
	}
	if _, err := os.Stat(defaultIdentityFile + ".pub"); err == nil {
		return sshIdentityMaterial{}, fmt.Errorf("managed SSH identity %s is incomplete; remove it or pass --identity-file", defaultIdentityFile)
	}
	if err := generateSSHIdentityFile(defaultIdentityFile); err != nil {
		return sshIdentityMaterial{}, err
	}
	material, err := loadSSHIdentityMaterial(defaultIdentityFile)
	return material, err
}

func loadSSHIdentityMaterial(identityFile string) (sshIdentityMaterial, error) {
	privateKey, publicKeyFile, err := resolveSSHIdentityFile(identityFile)
	if err != nil {
		return sshIdentityMaterial{}, err
	}
	publicKeyData, err := os.ReadFile(filepath.Clean(publicKeyFile))
	if err != nil {
		return sshIdentityMaterial{}, err
	}
	publicKey := normalizeAuthorizedKey(string(publicKeyData))
	if publicKey == "" {
		return sshIdentityMaterial{}, errors.New("SSH public key is empty")
	}
	return sshIdentityMaterial{
		IdentityFile:  privateKey,
		PublicKeyFile: publicKeyFile,
		PublicKey:     publicKey,
	}, nil
}

func defaultManagedSSHIdentityFile() (string, error) {
	home, err := lookupUserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return "", errors.New("could not resolve a default SSH identity path; pass --identity-file")
	}
	return filepath.Join(home, ".ssh", defaultManagedSSHIdentityName), nil
}

func generateSSHIdentityFile(identityFile string) error {
	identityFile = strings.TrimSpace(identityFile)
	if identityFile == "" {
		return errors.New("SSH identity file path is required")
	}
	if err := os.MkdirAll(filepath.Dir(identityFile), 0o700); err != nil {
		return err
	}
	sshKeygenPath, err := exec.LookPath("ssh-keygen")
	if err != nil {
		return err
	}
	comment := "spaces"
	if hostname, hostErr := os.Hostname(); hostErr == nil && strings.TrimSpace(hostname) != "" {
		comment = "spaces@" + strings.TrimSpace(hostname)
	}
	cmd := exec.Command(sshKeygenPath, "-q", "-t", "ed25519", "-N", "", "-f", identityFile, "-C", comment)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ssh-keygen failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func ensureSSHKeyRegistered(client apiClient, material sshIdentityMaterial) (sshKeyRecord, bool, error) {
	var listResponse struct {
		OK    bool           `json:"ok"`
		Error string         `json:"error"`
		Keys  []sshKeyRecord `json:"keys"`
	}
	if err := client.doJSON("GET", "/api/v1/ssh/keys", nil, &listResponse); err != nil {
		return sshKeyRecord{}, false, err
	}
	normalizedKey := normalizeAuthorizedKey(material.PublicKey)
	for _, key := range listResponse.Keys {
		if normalizeAuthorizedKey(key.PublicKey) == normalizedKey {
			return key, false, nil
		}
	}
	var addResponse struct {
		OK    bool         `json:"ok"`
		Error string       `json:"error"`
		Key   sshKeyRecord `json:"key"`
	}
	if err := client.doJSON("POST", "/api/v1/ssh/keys", map[string]any{
		"name":       defaultSSHKeyName(material.IdentityFile),
		"public_key": material.PublicKey,
	}, &addResponse); err != nil {
		return sshKeyRecord{}, false, err
	}
	return addResponse.Key, true, nil
}

func defaultSSHKeyName(identityFile string) string {
	base := strings.TrimSpace(filepath.Base(identityFile))
	base = strings.TrimSuffix(base, filepath.Ext(base))
	if base == "" {
		return "spaces"
	}
	return base
}

func normalizeAuthorizedKey(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

func resolvedKnownHostsFile(explicitPath string) (string, error) {
	if strings.TrimSpace(explicitPath) != "" {
		return strings.TrimSpace(explicitPath), nil
	}
	if envPath := strings.TrimSpace(os.Getenv("SPACES_SSH_KNOWN_HOSTS_FILE")); envPath != "" {
		return envPath, nil
	}
	home, err := lookupUserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return "", errors.New("could not resolve a default known_hosts path; pass --known-hosts-file")
	}
	return filepath.Join(home, ".ssh", defaultManagedKnownHostsName), nil
}

func fetchSSHKnownHosts(client apiClient, host string, port int) (sshKnownHostsRecord, error) {
	query := url.Values{}
	query.Set("host", host)
	query.Set("port", fmt.Sprintf("%d", port))
	var response struct {
		OK             bool   `json:"ok"`
		Error          string `json:"error"`
		Host           string `json:"host"`
		Port           int    `json:"port"`
		PublicKey      string `json:"public_key"`
		KnownHostsLine string `json:"known_hosts_line"`
	}
	if err := client.doJSONQuery("GET", "/api/v1/ssh/known-hosts", query, nil, &response); err != nil {
		return sshKnownHostsRecord{}, err
	}
	return sshKnownHostsRecord{
		Host:           response.Host,
		Port:           response.Port,
		PublicKey:      response.PublicKey,
		KnownHostsLine: response.KnownHostsLine,
	}, nil
}

func ensureSSHKnownHost(client apiClient, host string, port int, explicitPath string) (string, error) {
	knownHostsFile, err := resolvedKnownHostsFile(explicitPath)
	if err != nil {
		return "", err
	}
	record, err := fetchSSHKnownHosts(client, host, port)
	if err != nil {
		return "", err
	}
	if err := upsertKnownHostsLine(knownHostsFile, knownHostsHostPattern(record.Host, record.Port), record.KnownHostsLine); err != nil {
		return "", err
	}
	return knownHostsFile, nil
}

func upsertKnownHostsLine(path, hostPattern, knownHostsLine string) error {
	lines := make([]string, 0, 8)
	if payload, err := os.ReadFile(filepath.Clean(path)); err == nil {
		for _, line := range strings.Split(strings.ReplaceAll(string(payload), "\r\n", "\n"), "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			if lineContainsKnownHost(line, hostPattern) {
				continue
			}
			lines = append(lines, line)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	lines = append(lines, strings.TrimSpace(knownHostsLine))
	return writePrivateFile(path, []byte(strings.Join(lines, "\n")+"\n"))
}

func lineContainsKnownHost(line, hostPattern string) bool {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return false
	}
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return false
	}
	for _, candidate := range strings.Split(fields[0], ",") {
		if candidate == hostPattern {
			return true
		}
	}
	return false
}

func knownHostsHostPattern(host string, port int) string {
	if port == 22 {
		return strings.TrimSpace(host)
	}
	return fmt.Sprintf("[%s]:%d", strings.TrimSpace(host), port)
}
