// Zig programming language module
//
// This module provides functions that can be used to build a Zig program.

package main

import (
	"context"
	"dagger/zig/internal/dagger"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// A module to download the latest Zig master tarball
type Zig struct{}

type ZigMaster struct {
	FileName string
	Tarball  string
}

func getZigMaster(ctx context.Context, platform string) (ZigMaster, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://ziglang.org/download/index.json", nil)
	if err != nil {
		return ZigMaster{}, fmt.Errorf("unable to create request for zig download index, err: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ZigMaster{}, fmt.Errorf("unable to download zig download index, err: %w", err)
	}
	defer resp.Body.Close()

	jsonData, err := io.ReadAll(resp.Body)
	if err != nil {
		return ZigMaster{}, fmt.Errorf("unable to read zig download index, err: %w", err)
	}

	data := make(map[string]any)
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return ZigMaster{}, fmt.Errorf("invalid download index JSON data, err: %w", err)
	}

	masterData := data["master"].(map[string]any)
	tarball := masterData[platform].(map[string]any)["tarball"].(string)

	fileName := tarball[len("https://ziglang.org/builds/") : len(tarball)-len(".tar.xz")]

	return ZigMaster{
		FileName: fileName,
		Tarball:  tarball,
	}, nil
}

func platformToZigPlatform(platform dagger.Platform) (string, error) {
	switch platform {
	case "linux/amd64":
		return "x86_64-linux", nil
	default:
		return "", fmt.Errorf("invalid platform %q", platform)
	}
}

// Returns a Debian-based container with Zig installed and available in PATH
func (m *Zig) Container(ctx context.Context,
	// +optional
	// +default="linux/amd64"
	platform dagger.Platform,
) (*dagger.Container, error) {
	zigPlatform, err := platformToZigPlatform(platform)
	if err != nil {
		return nil, err
	}

	zigMaster, err := getZigMaster(ctx, zigPlatform)
	if err != nil {
		return nil, err
	}

	//

	ctr := dag.Container().
		From("debian:bookworm-slim").
		WithWorkdir("/app").
		// Fetch and install zig
		WithExec([]string{"apt-get", "update"}).
		WithExec([]string{"apt-get", "install", "-y", "curl", "xz-utils"}).
		WithExec([]string{"curl", "-J", "-o", "zig.tar.xz", zigMaster.Tarball}).
		WithExec([]string{"tar", "xJf", "zig.tar.xz"}).
		WithExec([]string{"mv", zigMaster.FileName, "zig-master"}).
		// Create a user and switch to it
		// WithExec([]string{"addgroup", "--gid", "1001", "zig"}).
		WithExec([]string{"adduser", "--gid", "100", "--uid", "1001", "zig"}).
		WithUser("zig").
		WithMountedCache("/home/zig/.cache/zig", dag.CacheVolume("global-zig-cache")).
		WithEnvVariable("PATH", "/usr/bin:/usr/sbin:/bin:/sbin:/app/zig-master")

	return ctr, nil
}
