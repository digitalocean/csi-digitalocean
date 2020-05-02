/*
Copyright 2020 DigitalOcean

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package driver

import (
	"context"

	"github.com/digitalocean/godo"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type godoLister func(ctx context.Context, listOpts *godo.ListOptions) ([]interface{}, *godo.Response, error)

func listResources(ctx context.Context, log *logrus.Entry, startingToken, maxEntries int32, lister godoLister) ([]interface{}, int32, error) {
	// Paginate through snapshots and return results.

	// Pagination is controlled by two request parameters:
	// MaxEntries indicates how many entries should be returned at most. If
	// more results are available, we must return a NextToken value
	// indicating the index for the next snapshot to request.
	// StartingToken defines the index of the first snapshot to return.
	// The CSI request parameters are defined in terms of number of
	// snapshots, not pages. It is up to the driver to translate the
	// parameters into paged requests accordingly.

	originalStartingToken := startingToken

	// Fetch snapshots until we have either collected maxEntries (if
	// positive) or all available ones, whichever comes first.
	listOpts := &godo.ListOptions{
		Page:    1,
		PerPage: int(maxEntries),
	}
	if maxEntries > 0 {
		// MaxEntries also defines the page size so that we can skip over
		// snapshots before the StartingToken and minimize the number of
		// paged requests we need.
		listOpts.Page = int(startingToken/maxEntries) + 1
		// Offset StartingToken to skip snapshots we do not want. This is
		// needed when MaxEntries does not divide StartingToken without
		// remainder.
		startingToken = startingToken % maxEntries
	}

	log = log.WithFields(logrus.Fields{
		"page":                    listOpts.Page,
		"computed_starting_token": startingToken,
	})

	var (
		// remainingEntries keeps track of how much room is left to return
		// as many as MaxEntries snapshots.
		remainingEntries = int(maxEntries)
		// hasMore indicates if NextToken must be set.
		hasMore   bool
		resources []interface{}
	)
	for {
		hasMore = false
		res, resp, err := lister(ctx, listOpts)
		if err != nil {
			return nil, 0, status.Errorf(codes.Internal, "listing resources failed: %s", err)
		}

		// Skip pre-StartingToken snapshots. This is required on the first
		// page at most.
		if startingToken > 0 {
			if startingToken > int32(len(res)) {
				startingToken = int32(len(res))
			} else {
				startingToken--
			}
			res = res[startingToken:]
		}
		startingToken = 0

		// Do not return more than MaxEntries across pages.
		if maxEntries > 0 && len(res) > remainingEntries {
			res = res[:remainingEntries]
			hasMore = true
		}

		resources = append(resources, res...)
		remainingEntries -= len(res)

		isLastPage := resp.Links == nil || resp.Links.IsLastPage()
		hasMore = hasMore || !isLastPage

		// Stop paging if we have used up all of MaxEntries.
		if maxEntries > 0 && remainingEntries == 0 {
			break
		}

		if isLastPage {
			break
		}

		page, err := resp.Links.CurrentPage()
		if err != nil {
			return nil, 0, err
		}

		listOpts.Page = page + 1
	}

	var nextToken int32
	if hasMore {
		// Compute NextToken, which is at least StartingToken plus
		// MaxEntries. If StartingToken was zero, we need to add one because
		// StartingToken defines the n-th snapshot we want but is not
		// zero-based.
		nextToken = originalStartingToken + maxEntries
		if originalStartingToken == 0 {
			nextToken++
		}
	}

	return resources, nextToken, nil
}
