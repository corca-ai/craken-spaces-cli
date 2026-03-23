package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallScriptVerifiesChecksums(t *testing.T) {
	releaseDir := t.TempDir()
	archiveName, archiveSHA := writeFakeReleaseArchive(t, releaseDir, "spaces")
	writeChecksumsFile(t, releaseDir, archiveName, archiveSHA)

	installDir := filepath.Join(t.TempDir(), "bin")
	output, err := runInstallScript(t, releaseDir, installDir, archiveSHA)
	if err != nil {
		t.Fatalf("install.sh failed: %v\n%s", err, output)
	}

	installedBinary := filepath.Join(installDir, "spaces")
	data, err := os.ReadFile(installedBinary)
	if err != nil {
		t.Fatalf("installed binary missing: %v", err)
	}
	if string(data) != "installed release\n" {
		t.Fatalf("installed binary contents=%q", string(data))
	}
}

func TestInstallScriptRejectsChecksumMismatch(t *testing.T) {
	releaseDir := t.TempDir()
	archiveName, archiveSHA := writeFakeReleaseArchive(t, releaseDir, "spaces")
	writeChecksumsFile(t, releaseDir, archiveName, "deadbeef")

	installDir := filepath.Join(t.TempDir(), "bin")
	output, err := runInstallScript(t, releaseDir, installDir, archiveSHA)
	if err == nil {
		t.Fatalf("expected install.sh to fail on checksum mismatch\n%s", output)
	}
	if _, statErr := os.Stat(filepath.Join(installDir, "spaces")); !os.IsNotExist(statErr) {
		t.Fatalf("binary should not have been installed, stat err=%v", statErr)
	}
}

func TestInstallDocsAvoidCurlPipeToSh(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		filepath.Join(wd, "..", "..", "README.md"),
		filepath.Join(wd, "..", "..", "specs", "index.spec.md"),
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%s): %v", path, err)
		}
		if strings.Contains(string(data), "| sh") {
			t.Fatalf("%s still contains curl-pipe-to-shell guidance", path)
		}
	}
}

func runInstallScript(t *testing.T, releaseDir, installDir, archiveSHA string) (string, error) {
	t.Helper()

	fakeBin := filepath.Join(t.TempDir(), "fakebin")
	if err := os.MkdirAll(fakeBin, 0o755); err != nil {
		t.Fatal(err)
	}

	writeExecutable(t, filepath.Join(fakeBin, "curl"), `#!/bin/sh
set -eu
out=""
url=""
while [ "$#" -gt 0 ]; do
	case "$1" in
		-o)
			out="$2"
			shift 2
			;;
		*)
			url="$1"
			shift
			;;
	esac
done

	case "$url" in
		*"/releases/latest")
			if [ -n "$out" ]; then
				cp "$FAKE_LATEST_JSON" "$out"
			else
				cat "$FAKE_LATEST_JSON"
			fi
			;;
		*)
			cp "$FAKE_RELEASE_DIR/$(basename "$url")" "$out"
			;;
	esac
`)
	writeExecutable(t, filepath.Join(fakeBin, "uname"), `#!/bin/sh
set -eu
case "$1" in
	-s) printf '%s\n' Linux ;;
	-m) printf '%s\n' x86_64 ;;
	*) exit 1 ;;
esac
`)
	writeExecutable(t, filepath.Join(fakeBin, "sha256sum"), fmt.Sprintf(`#!/bin/sh
set -eu
printf '%%s  %%s\n' '%s' "$1"
`, archiveSHA))
	writeExecutable(t, filepath.Join(fakeBin, "install"), `#!/bin/sh
set -eu
cp "$1" "$2"
chmod 755 "$2"
`)

	latestJSON := filepath.Join(releaseDir, "latest.json")
	if err := os.WriteFile(latestJSON, []byte(`{"tag_name":"v1.2.3"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sh", filepath.Join(wd, "..", "..", "install.sh"))
	cmd.Env = append(os.Environ(),
		"PATH="+fakeBin+":"+os.Getenv("PATH"),
		"FAKE_RELEASE_DIR="+releaseDir,
		"FAKE_LATEST_JSON="+latestJSON,
		"INSTALL_DIR="+installDir,
	)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func writeFakeReleaseArchive(t *testing.T, dir, binaryName string) (archiveName, archiveSHA string) {
	t.Helper()

	archiveName = "craken_1.2.3_linux_amd64.tar.gz"
	archivePath := filepath.Join(dir, archiveName)
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	gzw := gzip.NewWriter(file)
	tw := tar.NewWriter(gzw)

	payload := []byte("installed release\n")
	if err := tw.WriteHeader(&tar.Header{
		Name: binaryName,
		Mode: 0o755,
		Size: int64(len(payload)),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	archiveSHA = fmt.Sprintf("%x", sum[:])
	return archiveName, archiveSHA
}

func writeChecksumsFile(t *testing.T, dir, archiveName, archiveSHA string) {
	t.Helper()
	contents := fmt.Sprintf("%s  %s\n", archiveSHA, archiveName)
	if err := os.WriteFile(filepath.Join(dir, "checksums.txt"), []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeExecutable(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o755); err != nil {
		t.Fatal(err)
	}
}
