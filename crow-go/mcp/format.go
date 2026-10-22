// Copyright 2026 Crow CI Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mcp

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"codefloe.com/crowci/crow/v6/crow-go/crow"
)

const (
	// DefaultLogTail is the number of log lines returned when no tail is given.
	DefaultLogTail = 200
	// MaxLogLines caps every log response, whatever the options.
	MaxLogLines = 500
	// MaxLineBytes caps a single log line.
	MaxLineBytes = 2048
)

// Control character ranges (C0, DEL, C1) and the C1-to-ESC offset.
const (
	c0End     = 0x20
	del       = 0x7f
	c1Start   = 0x80
	c1End     = 0x9f
	c1ToFeOff = 0x40
)

// maxFieldBytes caps single-line fields such as titles and error messages.
const maxFieldBytes = 300

// escapeSequence matches terminal escape sequences: CSI, OSC (BEL or ST
// terminated), DCS/SOS/PM/APC (ST terminated) and any other ESC + one byte.
var escapeSequence = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]|\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)|\x1b[PX^_][^\x1b]*\x1b\\|\x1b.?`)

// delimiterText matches the untrusted-log delimiter in any letter case.
var delimiterText = regexp.MustCompile(`(?i)untrusted-log`)

// sanitize removes escape sequences and control characters from untrusted
// text and defuses the untrusted-log delimiter. Newlines become spaces.
func sanitize(s string) string {
	// Map raw C1 bytes (invalid UTF-8) to their 7-bit ESC equivalents so the
	// escape-sequence pass sees them; drop any other invalid bytes.
	if !utf8.ValidString(s) {
		var b strings.Builder
		for i := 0; i < len(s); {
			r, size := utf8.DecodeRuneInString(s[i:])
			if r == utf8.RuneError && size == 1 {
				if c := s[i]; c >= c1Start && c <= c1End {
					b.WriteByte('\x1b')
					b.WriteByte(c - c1ToFeOff)
				}
			} else {
				b.WriteString(s[i : i+size])
			}
			i += size
		}
		s = b.String()
	}
	s = escapeSequence.ReplaceAllString(s, "")
	s = strings.Map(func(r rune) rune {
		switch {
		case r == '\n':
			return ' '
		case r == '\t':
			return r
		case r < c0End, r == del, r >= c1Start && r <= c1End:
			return -1
		}
		return r
	}, s)
	return delimiterText.ReplaceAllString(s, "untrusted_log")
}

// truncate cuts s to at most n bytes on a rune boundary.
func truncate(s string, n int, marker string) string {
	if len(s) <= n {
		return s
	}
	cut := n
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + marker
}

// clean prepares an untrusted single-line field for output.
func clean(s string) string {
	return truncate(strings.TrimSpace(sanitize(s)), maxFieldBytes, "...")
}

func stepDuration(started, stopped int64) string {
	if started == 0 || stopped < started {
		return ""
	}
	return (time.Duration(stopped-started) * time.Second).String()
}

// FormatPipeline renders a pipeline summary with failed steps first.
func FormatPipeline(res *Resolved, p *crow.Pipeline) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", FormatPipelineRow(res, p))
	if p.Title != "" {
		fmt.Fprintf(&b, "title: %s\n", clean(p.Title))
	}
	for _, e := range p.Errors {
		kind := "error"
		if e.IsWarning {
			kind = "warning"
		}
		fmt.Fprintf(&b, "%s (%s): %s\n", kind, clean(e.Type), clean(e.Message))
	}

	var failed, other []string
	for _, wf := range currentWorkflows(p) {
		if len(wf.Children) == 0 && isFailed(wf.State) {
			line := fmt.Sprintf("  %s %s", clean(wf.Name), wf.State)
			if wf.Error != "" {
				line += ": " + clean(wf.Error)
			}
			failed = append(failed, line)
		}
		for _, s := range wf.Children {
			line := fmt.Sprintf("  %s/%s %s", clean(wf.Name), clean(s.Name), s.State)
			if isFailed(s.State) {
				line += fmt.Sprintf(" exit %d", s.ExitCode)
			}
			if d := stepDuration(s.Started, s.Stopped); d != "" {
				line += " " + d
			}
			if s.Error != "" {
				line += ": " + clean(s.Error)
			}
			if isFailed(s.State) {
				failed = append(failed, line+" "+res.Link(p, s.PID))
			} else {
				other = append(other, line)
			}
		}
	}
	if len(failed) > 0 {
		b.WriteString("failed:\n")
		b.WriteString(strings.Join(failed, "\n"))
		b.WriteString("\n")
	}
	if len(other) > 0 {
		b.WriteString("steps:\n")
		b.WriteString(strings.Join(other, "\n"))
		b.WriteString("\n")
	}
	return b.String()
}

// FormatPipelineRow renders a one-line pipeline summary.
func FormatPipelineRow(res *Resolved, p *crow.Pipeline) string {
	branch := p.Branch
	if isPREvent(p.Event) && p.Refspec != "" {
		branch = p.Refspec
	}
	row := fmt.Sprintf("#%d %s %s %s %s", p.Number, p.Status, p.Event, clean(branch), shortSHA(p.Commit))
	if p.Started > 0 {
		row += " started " + time.Unix(p.Started, 0).UTC().Format(time.RFC3339)
	}
	return row + " " + res.Link(p, 0)
}

// currentWorkflows drops workflow attempts that were superseded by a re-run,
// keeping the latest attempt of each workflow in the original order.
func currentWorkflows(p *crow.Pipeline) []*crow.Workflow {
	key := func(wf *crow.Workflow) int64 {
		if wf.OriginalWorkflowID != 0 {
			return wf.OriginalWorkflowID
		}
		return wf.ID
	}
	latest := map[int64]*crow.Workflow{}
	for _, wf := range p.Workflows {
		if prev, ok := latest[key(wf)]; !ok || wf.Attempt > prev.Attempt {
			latest[key(wf)] = wf
		}
	}
	out := make([]*crow.Workflow, 0, len(latest))
	for _, wf := range p.Workflows {
		if latest[key(wf)] == wf {
			out = append(out, wf)
		}
	}
	return out
}

// PickStep selects a step by PID or name. With an empty selector it returns
// the first failed step, else the first running step, else the last step.
func PickStep(p *crow.Pipeline, step string) (*crow.Step, error) {
	var all []*crow.Step
	for _, wf := range currentWorkflows(p) {
		all = append(all, wf.Children...)
	}
	if len(all) == 0 {
		return nil, fmt.Errorf("pipeline #%d has no steps yet", p.Number)
	}

	if step != "" {
		if pid, err := strconv.Atoi(step); err == nil {
			for _, s := range all {
				if s.PID == pid {
					return s, nil
				}
			}
		}
		for _, s := range all {
			if s.Name == step {
				return s, nil
			}
		}
		return nil, fmt.Errorf("pipeline #%d has no step %q", p.Number, step)
	}

	for _, s := range all {
		if isFailed(s.State) {
			return s, nil
		}
	}
	for _, s := range all {
		if s.State == crow.StatusRunning {
			return s, nil
		}
	}
	return all[len(all)-1], nil
}

// LogOptions selects which part of a step log to return.
type LogOptions struct {
	Tail    int
	Grep    string
	Context int
}

// FormatLog renders the selected part of a step log, delimited as untrusted
// content. With Grep set the whole log is searched and Tail is ignored.
func FormatLog(stepName string, entries []*crow.LogEntry, opt LogOptions) (string, error) {
	var lines []logLine
	for _, e := range entries {
		if e.Type != crow.LogEntryStdout && e.Type != crow.LogEntryStderr {
			continue
		}
		text := sanitize(strings.TrimRight(string(e.Data), "\r\n"))
		lines = append(lines, logLine{num: e.Line, text: truncate(text, MaxLineBytes, " ... (line truncated)")})
	}

	var (
		selected []string
		dropped  int
		header   string
	)
	if opt.Grep != "" {
		re, err := regexp.Compile(opt.Grep)
		if err != nil {
			return "", fmt.Errorf("invalid grep pattern: %w", err)
		}
		selected = grepLines(lines, re, max(opt.Context, 0))
		header = fmt.Sprintf("grep %q", opt.Grep)
		if len(selected) > MaxLogLines {
			dropped = len(selected) - MaxLogLines
			selected = selected[:MaxLogLines]
		}
	} else {
		tail := opt.Tail
		if tail <= 0 {
			tail = DefaultLogTail
		}
		tail = min(tail, MaxLogLines)
		start := max(len(lines)-tail, 0)
		for _, l := range lines[start:] {
			selected = append(selected, l.String())
		}
		header = fmt.Sprintf("last %d of %d lines", len(selected), len(lines))
	}

	var b strings.Builder
	fmt.Fprintf(&b, "<untrusted-log step=%q>\n", clean(stepName))
	fmt.Fprintf(&b, "(%s)\n", header)
	for _, l := range selected {
		b.WriteString(l)
		b.WriteString("\n")
	}
	if dropped > 0 {
		fmt.Fprintf(&b, "... truncated (%d more lines)\n", dropped)
	}
	b.WriteString("</untrusted-log>\n")
	return b.String(), nil
}

type logLine struct {
	num  int
	text string
}

func (l logLine) String() string {
	return fmt.Sprintf("%d: %s", l.num, l.text)
}

// grepLines matches re against the log text (not the line-number prefix).
func grepLines(lines []logLine, re *regexp.Regexp, context int) []string {
	keep := make([]bool, len(lines))
	for i, l := range lines {
		if re.MatchString(l.text) {
			for j := max(i-context, 0); j <= min(i+context, len(lines)-1); j++ {
				keep[j] = true
			}
		}
	}
	var out []string
	prev := -1
	for i, k := range keep {
		if !k {
			continue
		}
		if prev >= 0 && i != prev+1 {
			out = append(out, "--")
		}
		out = append(out, lines[i].String())
		prev = i
	}
	return out
}
