package main

import (
	"errors"
	"strings"

	"github.com/undervoke/teakit"
)

type diffExample struct {
	name, title string
	files       []teakit.DiffFile
	apply, skip string
	accept      func(apply bool) error
}

var diffExamples = []diffExample{
	{
		name:  "One value",
		title: "Review the change",
		files: []teakit.DiffFile{{
			Path: "~/.claude/settings.json",
			Sections: []teakit.DiffSection{{
				Label: "/statusLine",
				Ops: diffLines(`
 {
-  "command": "ccstatusline",
+  "command": "coa statusline --compact",
   "type": "command"
 }`),
			}},
		}},
	},
	{
		name:  "Several files, longer than the screen",
		title: "Review the update",
		files: []teakit.DiffFile{
			{
				Path: "~/.claude/settings.json",
				Sections: []teakit.DiffSection{
					{
						Label: "/permissions",
						Notes: []string{"2 comment(s) inside this block are removed"},
						Ops: diffLines(`
 {
   "allow": [
     "Bash(go build:*)",
-    "Bash(go test:*)",
-    "Bash(go vet:*)"
+    "Bash(go test:*)"
   ],
+  "ask": ["Bash(git push:*)"],
   "deny": [
     "Read(./.env)"
   ]
 }`),
					},
					{
						Label:        "/experimental",
						BeforeAbsent: true,
						Ops:          diffLines(`+"experimental": true`),
					},
					{
						Label:       "/legacyTheme",
						AfterAbsent: true,
						Ops: diffLines(`
-{
-  "name": "solarized",
-  "contrast": "high"
-}`),
					},
				},
			},
			{
				Path: "~/.config/runner/config.toml",
				Sections: []teakit.DiffSection{{
					Label: "runner",
					Ops: diffLines(`
 [runner]
 name = "build"
-labels = ["macOS"]
+labels = ["macOS", "ARM64", "xcode"]
 idle = "10m"

 [vm]
-cpus = 4
-memory = "8GiB"
+cpus = 8
+memory = "16GiB"
 disk = "64GiB"

 [cache]
+enabled = true
 path = "~/Library/Caches/runner"`),
				}},
			},
			{
				Path: "~/Library/LaunchAgents/com.example.runner.plist",
				Sections: []teakit.DiffSection{{
					Label:        "new file",
					BeforeAbsent: true,
					Ops: diffLines(`
+<?xml version="1.0" encoding="UTF-8"?>
+<plist version="1.0">
+<dict>
+  <key>Label</key>
+  <string>com.example.runner</string>
+  <key>ProgramArguments</key>
+  <array>
+    <string>/usr/local/bin/runner</string>
+    <string>serve</string>
+  </array>
+  <key>RunAtLoad</key>
+  <true/>
+  <key>KeepAlive</key>
+  <true/>
+</dict>
+</plist>`),
				}},
			},
			{
				Path: "~/.runner-old.yml",
				Sections: []teakit.DiffSection{{
					Label:       "removed file",
					AfterAbsent: true,
					Ops: diffLines(`
-name: build
-labels: macOS
-idle: 10m`),
				}},
			},
		},
	},
	{
		name:  "Long lines and other scripts",
		title: "Review the change",
		files: []teakit.DiffFile{{
			Path: "~/projects/example/a-deep/directory/with-a-long-name/and-another/hooks.json",
			Sections: []teakit.DiffSection{{
				Label: "/hooks/SessionStart",
				Ops: diffLines(`
 {
-  "command": "/usr/local/bin/example-agent hook session-start --host claude --format json --timeout 30s",
+  "command": "/usr/local/bin/example-agent hook session-start --host claude --format json --timeout 60s --retry 3",
-  "description": "세션이 시작될 때 실행합니다",
+  "description": "세션이 시작되거나 다시 열릴 때 실행합니다. 실패해도 세션은 계속됩니다",
   "enabled": true
 }`),
			}},
		}},
	},
	{
		name:  "A change that must be applied",
		title: "Review the update",
		accept: func(apply bool) error {
			if !apply {
				return errors.New("The update cannot go on without this. Apply it, or leave.")
			}
			return nil
		},
		files: []teakit.DiffFile{{
			Path: "~/.local/share/runner/state.db",
			Sections: []teakit.DiffSection{{
				Label:       "jobs (3 rows)",
				Notes:       []string{"no backup is made"},
				AfterAbsent: true,
				Ops: diffLines(`
-1041  build   passed   2m10s
-1042  test    failed   5m48s
-1043  deploy  passed   0m52s`),
			}},
		}, {
			Path: "~/.local/share/runner/schema",
			Sections: []teakit.DiffSection{{
				Ops: diffLines(`
-version 7
+version 8`),
			}},
		}},
	},
	{
		name:  "Restore or keep",
		title: "Restore the values these settings replaced?",
		apply: "restore",
		skip:  "keep current",
		files: []teakit.DiffFile{{
			Path: "~/.zshrc",
			Sections: []teakit.DiffSection{{
				Label: "EDITOR",
				Ops: diffLines(`
-export EDITOR="runner edit"
+export EDITOR="vim"`),
			}},
		}, {
			Path: "~/.gitconfig",
			Sections: []teakit.DiffSection{{
				Label: "core.pager",
				Ops: diffLines(`
 [core]
-  pager = runner pager
+  pager = less -FRX`),
			}},
		}},
	},
}

// diffLines reads a diff written as text: a line's first character says
// whether it is kept, removed or added.
func diffLines(text string) []teakit.DiffOp {
	var ops []teakit.DiffOp
	for _, line := range strings.Split(strings.TrimPrefix(text, "\n"), "\n") {
		op := teakit.DiffOp{Kind: teakit.OpEqual}
		if line != "" {
			op.Text = line[1:]
			switch line[0] {
			case '-':
				op.Kind = teakit.OpDelete
			case '+':
				op.Kind = teakit.OpInsert
			}
		}
		ops = append(ops, op)
	}
	return ops
}
