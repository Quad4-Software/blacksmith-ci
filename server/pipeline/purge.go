package pipeline

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/rs/zerolog/log"

	"codeberg.org/crowci/crow/v3/server/model"
	"codeberg.org/crowci/crow/v3/server/store"
)

func pipelineLogPurge(store store.Store, repo *model.Repo) error {
	olderThanDuration := getLogRetentionDuration(repo)

	var pipelinesToPurge []*model.Pipeline
	var err error

	// Fetch pipelines based on retention settings
	pipelinesToPurge, err = fetchFilteredPipelines(store, repo, olderThanDuration, repo.LogsPipelinesKeepMin)
	if err != nil {
		return fmt.Errorf("failed to fetch pipelines for purging: %w", err)
	}

	for _, pipeline := range pipelinesToPurge {
		// check if logs were already purged
		logsPurged, err := store.GetPipelineLogPurged(pipeline)
		log.Trace().Msgf("Logs for pipeline %d have already been purged.", pipeline.Number)
		if err != nil {
			return fmt.Errorf("failed to check if pipeline %d logs are purged: %w", pipeline.Number, err)
		}
		if logsPurged {
			continue
		}
		if err := purgePipelineLogs(store, pipeline); err != nil {
			log.Error().Err(err).Msgf("Failed to purge logs for pipeline %d", pipeline.Number)
		}
		// prevents additional purge calls in the future
		if err := store.SetPipelineLogPurged(pipeline); err != nil {
			log.Trace().Msgf("Setting 'logs_purged' indicator for pipeline %d", pipeline.Number)
			log.Error().Err(err).Msgf("Failed to mark pipeline %d logs as purged", pipeline.Number)
		}
	}

	return nil
}

// getLogRetentionDuration extracts and validates log retention duration.
func getLogRetentionDuration(repo *model.Repo) time.Duration {
	if repo.LogsDurationKeep == "" {
		return 0
	}

	duration, err := time.ParseDuration(repo.LogsDurationKeep)
	if err != nil {
		log.Error().Err(err).Msg("Invalid LogsDurationKeep value, defaulting to 0")
		return 0
	}
	return duration
}

// purgePipelineLogs deletes logs for all steps in a given pipeline.
func purgePipelineLogs(store store.Store, pipeline *model.Pipeline) error {
	steps, err := store.StepList(pipeline)
	if err != nil {
		return fmt.Errorf("failed to fetch steps for pipeline %d: %w", pipeline.Number, err)
	}

	for _, step := range steps {
		log.Debug().Msgf("Deleting logs for step %d in pipeline %d (repo %d)", step.ID, pipeline.Number, pipeline.RepoID)
		if err := store.LogDelete(step); err != nil {
			log.Error().Err(err).Msgf("Failed to delete logs for step %d in pipeline %d", step.ID, pipeline.Number)
			continue
		}
	}

	return nil
}

func fetchFilteredPipelines(store store.Store, repo *model.Repo, olderThan time.Duration, keepMin int64) ([]*model.Pipeline, error) {
	const batchSize = 100
	beforeDate := time.Now().Add(-olderThan).Unix()
	var allPipelines []*model.Pipeline

	if keepMin < 0 {
		log.Debug().Msgf("keepMin is negative (%d), treating it as zero", keepMin)
		keepMin = 0
	}

	// First, get total count to handle keepMin requirement
	totalCount, err := store.GetPipelineCountByRepo(repo.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get pipeline count: %w", err)
	}

	// If total count is less than or equal to keepMin, return early
	if totalCount <= keepMin {
		log.Debug().Msgf("Total pipelines (%d) <= keepMin (%d), skipping purge", totalCount, keepMin)
		return nil, nil
	}

	// Calculate how many pipelines we can purge while respecting keepMin
	purgeLimit := totalCount - keepMin

	// Use keyset pagination with cursor
	var (
		lastID    int64 = math.MaxInt64
		processed int64
	)

	for processed < totalCount {
		batch, err := store.GetPipelineListKeyset(
			repo,
			lastID,
			beforeDate,
			batchSize,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch pipeline batch: %w", err)
		}

		// Break if no more results
		if len(batch) == 0 {
			break
		}

		// Update lastID for next iteration
		lastID = batch[len(batch)-1].ID
		processed += int64(len(batch))

		// Filter and append pipelines
		for _, p := range batch {
			// Apply olderThan filter if specified
			if olderThan > 0 && p.Created >= beforeDate {
				continue
			}
			allPipelines = append(allPipelines, p)
		}

		log.Debug().
			Int64("processed", processed).
			Int64("total", totalCount).
			Int64("keepMin", keepMin).
			Msgf("Processed pipeline batch")

		if processed >= totalCount {
			break
		}
	}

	sort.Slice(allPipelines, func(i, j int) bool {
		return allPipelines[i].Created < allPipelines[j].Created
	})
	log.Debug().Msgf("Oldest pipeline ID for purging (before filtering): %d", allPipelines[0].ID)
	log.Debug().Msgf("Most recent pipeline ID for purging (before filtering): %d", allPipelines[len(allPipelines)-1].ID)
	log.Debug().Msgf("Purge limit: %d", purgeLimit)
	log.Debug().Msgf("len(allPipelines): %d", len(allPipelines))

	if int64(len(allPipelines)) > purgeLimit {
		allPipelines = allPipelines[:purgeLimit]
	}

	log.Debug().Msgf("Oldest pipeline ID for purging (after filtering): %d", allPipelines[0].ID)
	log.Debug().Msgf("Most recent pipeline ID for purging (after filtering): %d", allPipelines[len(allPipelines)-1].ID)

	if len(allPipelines) > 0 {
		log.Debug().
			Int("count", len(allPipelines)).
			Dur("olderThan", olderThan).
			Int64("keepMin", keepMin).
			Msgf("Possible pipelines eligible for log purging")
	}

	return allPipelines, nil
}
