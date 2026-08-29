// Copyright 2026 Crow Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package common

import (
	"bufio"
	"io"
	"sync"

	"github.com/rs/zerolog/log"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

// TaggedMergedPipe creates a pipe that merges stdout and stderr with stream
// tags, so a single reader can carry both while the consumer still knows which
// stream each line came from.
// Tags: 0x01 = stdout, 0x02 = stderr.
func TaggedMergedPipe(stdout, stderr io.ReadCloser, isWindows bool) io.ReadCloser {
	pr, pw := io.Pipe()

	go func() {
		var wg sync.WaitGroup
		const numStreams = 2 // stdout and stderr
		wg.Add(numStreams)

		// Copy stdout with tag 0x01
		go func() {
			defer wg.Done()
			reader := stdout
			if isWindows {
				// Sanitize non-UTF8 output on Windows
				reader = io.NopCloser(transform.NewReader(stdout, unicode.UTF8.NewDecoder().Transformer))
			}
			copyWithTag(pw, reader, '\x01')
			stdout.Close()
		}()

		// Copy stderr with tag 0x02
		go func() {
			defer wg.Done()
			reader := stderr
			if isWindows {
				// Sanitize non-UTF8 output on Windows
				reader = io.NopCloser(transform.NewReader(stderr, unicode.UTF8.NewDecoder().Transformer))
			}
			copyWithTag(pw, reader, '\x02')
			stderr.Close()
		}()

		wg.Wait()
		pw.Close()
	}()

	return pr
}

// copyWithTag reads from src line-by-line and writes to dst, prepending each
// line with a tag byte.
func copyWithTag(dst io.Writer, src io.Reader, tag byte) {
	scanner := bufio.NewScanner(src)
	for scanner.Scan() {
		line := scanner.Bytes()
		// Prepend tag byte to each line
		tagged := make([]byte, len(line)+2) // +1 for tag, +1 for newline
		tagged[0] = tag
		copy(tagged[1:], line)
		tagged[len(tagged)-1] = '\n'
		if _, err := dst.Write(tagged); err != nil {
			log.Error().Err(err).Msg("failed to write tagged log line")
			return
		}
	}
}
