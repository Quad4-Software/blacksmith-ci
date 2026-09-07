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

package internal

import "fmt"

// maxPages bounds how many pages a single collection may span. Terminating on
// the `next` link already ends every well-behaved collection, so this is purely
// a safety net against an endpoint that advertises a next page forever; the
// bound is therefore set well beyond any realistic collection size.
const maxPages = 1000

// paginate walks a Bitbucket Cloud paged collection.
//
// Every paged response carries the absolute URL of the following page in its
// `next` field and omits that field on the last page, so `next` is the only
// reliable stop condition. Bitbucket does not honor `page` on all collections
// (`/hooks` most notably ignores it and answers every request with the same
// first page), which makes any "stop once a page comes back short" heuristic
// spin forever. See issue #1487.
func paginate[T any](get func(opts *ListOpts) (values []T, next string, err error)) ([]T, error) {
	items := make([]T, 0, pageSize)
	opts := &ListOpts{Page: 1, PageLen: pageSize}

	for opts.Page <= maxPages {
		values, next, err := get(opts)
		if err != nil {
			return nil, err
		}
		items = append(items, values...)

		if next == "" {
			return items, nil
		}
		opts.Page++
	}

	return nil, fmt.Errorf("bitbucket pagination did not terminate after %d pages", maxPages)
}
