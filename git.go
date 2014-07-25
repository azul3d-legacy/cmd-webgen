// Copyright 2014 The Azul3D Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"log"
	"os"
	"os/exec"
)

// runs "git add -A ." in workingDir.
func gitadda(workingDir string) error {
	log.Printf("    git add -A .\n")
	cmd := exec.Command("git", "add", "-A", ".")
	cmd.Dir = workingDir
	cmd.Stdout = prefixWriter{out: os.Stdout, prefix: []byte("        ")}
	cmd.Stderr = prefixWriter{out: os.Stderr, prefix: []byte("        ")}
	return cmd.Run()
}

// runs "git commit -a -m <message> <path>" in workingDir.
func gitcommitam(workingDir, message string) error {
	log.Printf("    git commit -a -m %q\n", message)
	cmd := exec.Command("git", "commit", "-a", "-m", message)
	cmd.Dir = workingDir
	cmd.Stdout = prefixWriter{out: os.Stdout, prefix: []byte("        ")}
	cmd.Stderr = prefixWriter{out: os.Stderr, prefix: []byte("        ")}
	return cmd.Run()
}

// runs "git push" in workingDir.
func gitpush(workingDir string) error {
	log.Printf("    git push\n")
	cmd := exec.Command("git", "push")
	cmd.Dir = workingDir
	cmd.Stdout = prefixWriter{out: os.Stdout, prefix: []byte("        ")}
	cmd.Stderr = prefixWriter{out: os.Stderr, prefix: []byte("        ")}
	return cmd.Run()
}
