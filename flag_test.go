//
// FilePath    : zap-smap\flag_test.go
// Author      : jiaopengzi
// Blog        : https://jiaopengzi.com
// Copyright   : Copyright (c) 2026 by jiaopengzi, All Rights Reserved.
// Description : 单测
//

package main

import (
	"flag"
	"testing"
)

func TestCheckFlagConflicts(t *testing.T) {
	orig := flag.CommandLine
	t.Cleanup(func() { flag.CommandLine = orig })

	cases := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{"none", []string{}, false},
		{"del-only", []string{"-del", "x"}, false},
		{"field-only", []string{"-field", "x"}, false},
		{"both", []string{"-del", "x", "-field", "y"}, true},
	}

	const wantMsg = "cannot use -del and -field at the same time; remove one flag or let -field use its default"

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			fs := flag.NewFlagSet("test", flag.ContinueOnError)
			fs.String("del", "", "")
			fs.String("field", "", "")

			if err := fs.Parse(tc.args); err != nil {
				t.Fatalf("parse args: %v", err)
			}

			flag.CommandLine = fs
			err := checkFlagConflicts()
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error but got nil")
				}
				if err.Error() != wantMsg {
					t.Fatalf("unexpected error message: %v", err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}
