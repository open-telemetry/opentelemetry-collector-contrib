// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build ignore

// gen.go regenerates partition.go and partitions.go from
// aws-sdk-go-v2's internal/endpoints/awsrulesfn package using the local
// Go module cache. Run: go generate ./override/aws/awsrulesfn/
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

const sdkModule = "github.com/aws/aws-sdk-go-v2"

func main() {
	out, err := exec.Command("go", "list", "-m", "-json", sdkModule).Output()
	if err != nil {
		die("go list: %v", err)
	}

	var info struct{ Path, Version, Dir string }
	if err := json.Unmarshal(out, &info); err != nil {
		die("parse go list output: %v", err)
	}

	src := filepath.Join(info.Dir, "internal", "endpoints", "awsrulesfn")
	for _, f := range []string{"partition.go", "partitions.go"} {
		if err := copyWithHeader(filepath.Join(src, f), f, info.Path, info.Version); err != nil {
			die("copy %s: %v", f, err)
		}
	}
	fmt.Printf("Regenerated awsrulesfn from %s@%s\n", info.Path, info.Version)
}

func copyWithHeader(srcPath, dstName, modPath, version string) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.Create(dstName)
	if err != nil {
		return err
	}
	defer dst.Close()
	if _, err := fmt.Fprintf(dst, "// Code generated from %s@%s/internal/endpoints/awsrulesfn. DO NOT EDIT.\n", modPath, version); err != nil {
		return err
	}
	_, err = io.Copy(dst, src)
	return err
}

func die(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "gen: "+format+"\n", args...)
	os.Exit(1)
}
