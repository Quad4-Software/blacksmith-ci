/* eslint-disable no-unused-vars */
export enum WebhookEvents {
  Push = 'push',
  Tag = 'tag',
  Release = 'release',
  PullRequest = 'pull_request',
  PullRequestClosed = 'pull_request_closed',
  PullRequestMerged = 'pull_request_merged',
  PullRequestEdited = 'pull_request_edited',
  Deploy = 'deployment',
  Cron = 'cron',
  Manual = 'manual',
}
/* eslint-enable */
