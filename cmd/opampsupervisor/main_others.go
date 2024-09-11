//go:build !windows

package main

func run() error {
	return runInteractive()
}
