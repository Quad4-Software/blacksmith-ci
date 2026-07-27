<template>
  <component
    :is="isFullscreen ? 'div' : 'div'"
    v-if="pipeline"
    ref="componentRef"
    :class="isFullscreen ? 'fixed inset-0 z-50 bg-background' : 'flex flex-col flex-1 min-h-0'"
  >
    <Card
      class="flex flex-col flex-1 min-h-0 overflow-hidden p-0 relative"
      :class="[logThemeClass, isFullscreen ? 'rounded-none border-0' : '']"
    >
      <!-- Desktop header - hidden on mobile since we have a mobile header in the parent -->
      <CardHeader class="hidden md:flex flex-row items-center justify-between px-3 py-2 border-b space-y-0 gap-3">
        <!-- Left: Navigation buttons -->
        <div class="flex items-center gap-2">
          <TooltipProvider>
            <Tooltip>
              <TooltipTrigger as-child>
                <Button
                  variant="ghost"
                  size="icon-sm"
                  :disabled="!previousStepId"
                  @click="emit('update:step-id', previousStepId!)"
                >
                  <Icon name="chevron-up" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>{{ $t('repo.pipeline.previous_step') }}</TooltipContent>
            </Tooltip>
          </TooltipProvider>
          <TooltipProvider>
            <Tooltip>
              <TooltipTrigger as-child>
                <Button
                  variant="ghost"
                  size="icon-sm"
                  :disabled="!nextStepId"
                  @click="emit('update:step-id', nextStepId!)"
                >
                  <Icon name="chevron-down" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>{{ $t('repo.pipeline.next_step') }}</TooltipContent>
            </Tooltip>
          </TooltipProvider>
        </div>

        <!-- Center: Step name, status, and exit code -->
        <div class="flex items-center gap-2 flex-1 min-w-0">
          <PipelineStatusIcon v-if="step" :status="step.state" class="h-4 w-4 shrink-0" />
          <span class="text-sm font-semibold text-foreground truncate"
            >{{ step?.name }}<span class="font-normal text-muted-foreground">{{ stepNameSuffix }}</span></span
          >
          <span
            v-if="step?.exit_code !== undefined"
            class="text-xs border rounded-full px-2 py-0.5 shrink-0"
            :class="step.exit_code === 0 ? 'border-green-500' : 'border-destructive'"
          >
            {{ $t('repo.pipeline.exit_code', { exitCode: step.exit_code }) }}
          </span>
          <!-- Attempt selector dropdown -->
          <!-- eslint-disable @intlify/vue-i18n/no-raw-text -->
          <DropdownMenu v-if="hasMultipleAttempts && currentWorkflow">
            <DropdownMenuTrigger as-child>
              <button
                class="inline-flex items-center gap-1 rounded-md border border-primary/70 px-2 py-0.5 text-xs text-muted-foreground hover:bg-accent transition-colors shrink-0"
              >
                #{{ currentWorkflow.attempt }}
                <Icon name="chevron-down" class="h-3 w-3" />
              </button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="start">
              <DropdownMenuItem
                v-for="attempt in workflowAttempts"
                :key="attempt.id"
                class="flex items-center gap-2 text-xs"
                @click="switchToAttempt(attempt)"
              >
                <PipelineStatusIcon :status="attempt.state" class="h-3.5 w-3.5" />
                #{{ attempt.attempt }}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
          <!-- eslint-enable @intlify/vue-i18n/no-raw-text -->
        </div>

        <!-- Right: Action buttons -->
        <div class="flex flex-row items-center gap-x-2 shrink-0">
          <TooltipProvider v-if="hasPushPermission && currentWorkflow && step && isStepCompleted">
            <Tooltip>
              <TooltipTrigger as-child>
                <Button variant="ghost" size="icon-sm" @click="emit('rerunStep', currentWorkflow!.id, step!.id)">
                  <Icon name="solar:restart-bold" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>{{ $t('repo.pipeline.actions.rerun_step') }}</TooltipContent>
            </Tooltip>
          </TooltipProvider>
          <TooltipProvider v-if="step?.finished !== undefined && hasLogs">
            <Tooltip>
              <TooltipTrigger as-child>
                <Button variant="ghost" size="icon-sm" :disabled="downloadInProgress" @click="download">
                  <Icon v-if="downloadInProgress" name="spinner" class="animate-spin" />
                  <Icon v-else name="download" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>
                {{ $t('repo.pipeline.actions.log_download') }}
              </TooltipContent>
            </Tooltip>
          </TooltipProvider>
          <TooltipProvider v-if="hasLogs">
            <Tooltip>
              <TooltipTrigger as-child>
                <Button variant="ghost" size="icon-sm" :disabled="copyInProgress" @click="copyLogs">
                  <Icon v-if="copyInProgress" name="spinner" class="animate-spin" />
                  <Icon v-else :name="logsCopied ? 'lucide:check' : 'lucide:copy'" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>
                {{ logsCopied ? $t('copied') : $t('repo.pipeline.actions.log_copy') }}
              </TooltipContent>
            </Tooltip>
          </TooltipProvider>
          <TooltipProvider v-if="step?.finished !== undefined && hasLogs && hasPushPermission">
            <Tooltip>
              <TooltipTrigger as-child>
                <Button variant="ghost" size="icon-sm" @click="showDeleteLogsDialog = true">
                  <Icon name="trash" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>
                {{ $t('repo.pipeline.actions.log_delete') }}
              </TooltipContent>
            </Tooltip>
          </TooltipProvider>
          <TooltipProvider v-if="step?.finished === undefined">
            <Tooltip>
              <TooltipTrigger as-child>
                <Button variant="ghost" size="icon-sm" @click="autoScroll = !autoScroll">
                  <Icon :name="autoScroll ? 'auto-scroll' : 'auto-scroll-off'" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>
                {{
                  autoScroll
                    ? $t('repo.pipeline.actions.log_auto_scroll_off')
                    : $t('repo.pipeline.actions.log_auto_scroll')
                }}
              </TooltipContent>
            </Tooltip>
          </TooltipProvider>
          <TooltipProvider>
            <Tooltip>
              <TooltipTrigger as-child>
                <Button variant="ghost" size="icon-sm" @click="openSearch">
                  <Icon name="lucide:search" class="w-4 h-4" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>{{ $t('repo.pipeline.search.title') }}</TooltipContent>
            </Tooltip>
          </TooltipProvider>
          <TooltipProvider>
            <Tooltip>
              <TooltipTrigger as-child>
                <Button variant="ghost" size="icon-sm" @click="isFullscreen = !isFullscreen">
                  <Icon :name="isFullscreen ? 'lucide:minimize' : 'lucide:maximize'" class="w-4 h-4" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>
                {{
                  isFullscreen
                    ? $t('repo.pipeline.actions.log_exit_fullscreen')
                    : $t('repo.pipeline.actions.log_fullscreen')
                }}
              </TooltipContent>
            </Tooltip>
          </TooltipProvider>
          <Popover v-model:open="settingsPopoverOpen">
            <PopoverTrigger as-child>
              <Button variant="ghost" size="icon-sm">
                <Icon name="settings" class="w-4 h-4" />
              </Button>
            </PopoverTrigger>
            <PopoverContent align="end" class="w-80">
              <div class="space-y-5 p-1">
                <div>
                  <h4 class="text-sm font-semibold">{{ $t('repo.pipeline.actions.log_settings') }}</h4>
                </div>

                <div class="space-y-2.5">
                  <label class="text-sm font-medium block">{{ $t('user.settings.general.log_theme') }}</label>
                  <Select v-model="logTheme">
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem v-for="option in logThemeOptions" :key="option.value" :value="option.value">
                        {{ option.text }}
                      </SelectItem>
                    </SelectContent>
                  </Select>
                </div>

                <div class="space-y-2.5">
                  <label class="text-sm font-medium block">{{
                    $t('user.settings.general.pipeline_steps_width')
                  }}</label>
                  <div class="flex items-center gap-3">
                    <NumberField v-model="pipelineSplitRatioTemp" :min="20" :max="50" class="max-w-[8rem]">
                      <NumberFieldContent>
                        <NumberFieldDecrement />
                        <NumberFieldInput />
                        <NumberFieldIncrement />
                      </NumberFieldContent>
                    </NumberField>
                    <span class="text-sm text-muted-foreground">
                      {{ $t('user.settings.general.split_ratio_range') }}
                    </span>
                  </div>
                </div>

                <div class="space-y-2.5">
                  <label class="text-sm font-medium block">
                    {{ $t('repo.pipeline.actions.log_line_limit') }}
                  </label>
                  <div class="flex items-center gap-3">
                    <NumberField v-model="logLineLimitTemp" :min="1000" :max="50000" :step="1000" class="max-w-[8rem]">
                      <NumberFieldContent>
                        <NumberFieldDecrement />
                        <NumberFieldInput />
                        <NumberFieldIncrement />
                      </NumberFieldContent>
                    </NumberField>
                    <span class="text-sm text-muted-foreground">
                      {{ $t('repo.pipeline.actions.log_line_limit_range') }}
                    </span>
                  </div>
                  <p class="text-xs text-muted-foreground">
                    {{ $t('repo.pipeline.actions.log_line_limit_desc') }}
                  </p>
                </div>

                <div class="flex items-center justify-between">
                  <div class="space-y-0.5">
                    <label class="text-sm font-medium text-foreground">{{
                      $t('repo.pipeline.actions.auto_follow')
                    }}</label>
                    <p class="text-xs text-muted-foreground">
                      {{ $t('repo.pipeline.actions.auto_follow_desc') }}
                    </p>
                  </div>
                  <Switch
                    :model-value="autoFollowEnabled"
                    @update:model-value="(value) => emit('update:auto-follow-enabled', value)"
                  />
                </div>
              </div>
            </PopoverContent>
          </Popover>
        </div>
      </CardHeader>

      <!-- Mobile action bar - inline between header and log content -->
      <div class="md:hidden flex items-center justify-end gap-1 px-2 py-1">
        <Button variant="ghost" size="icon-sm" @click="openSearch">
          <Icon name="lucide:search" />
        </Button>
        <Button variant="ghost" size="icon-sm" @click="mobileSettingsOpen = true">
          <Icon name="settings" class="w-4 h-4" />
        </Button>
      </div>

      <!-- Mobile floating scroll buttons -->
      <div class="md:hidden fixed bottom-20 right-3 z-50 flex flex-col gap-2">
        <Button
          v-if="!atTop"
          variant="outline"
          size="icon-sm"
          class="rounded-full shadow-lg bg-background dark:bg-background"
          @click="scrollToTop"
        >
          <Icon name="chevron-up" />
        </Button>
        <Button
          v-if="!atBottom"
          variant="outline"
          size="icon-sm"
          class="rounded-full shadow-lg bg-background dark:bg-background"
          @click="scrollToBottom"
        >
          <Icon name="chevron-down" />
        </Button>
      </div>

      <!-- Mobile Settings Sheet -->
      <Sheet v-model:open="mobileSettingsOpen">
        <SheetContent side="bottom" class="h-auto max-h-[80vh]">
          <SheetHeader>
            <SheetTitle>{{ $t('repo.pipeline.actions.log_settings') }}</SheetTitle>
          </SheetHeader>
          <div class="space-y-6 px-4 py-4">
            <!-- Log Theme -->
            <div class="space-y-2">
              <label class="text-sm font-medium block">{{ $t('user.settings.general.log_theme') }}</label>
              <Select v-model="logTheme">
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem v-for="option in logThemeOptions" :key="option.value" :value="option.value">
                    {{ option.text }}
                  </SelectItem>
                </SelectContent>
              </Select>
            </div>

            <!-- Log Line Limit -->
            <div class="space-y-2">
              <label class="text-sm font-medium block">{{ $t('repo.pipeline.actions.log_line_limit') }}</label>
              <NumberField v-model="logLineLimit" :min="1000" :max="50000" :step="1000" class="w-full">
                <NumberFieldContent>
                  <NumberFieldDecrement />
                  <NumberFieldInput />
                  <NumberFieldIncrement />
                </NumberFieldContent>
              </NumberField>
              <p class="text-xs text-muted-foreground">{{ $t('repo.pipeline.actions.log_line_limit_desc') }}</p>
            </div>

            <!-- Auto Follow -->
            <div class="flex items-center justify-between">
              <div class="space-y-0.5">
                <label class="text-sm font-medium text-foreground">{{ $t('repo.pipeline.actions.auto_follow') }}</label>
                <p class="text-xs text-muted-foreground">{{ $t('repo.pipeline.actions.auto_follow_desc') }}</p>
              </div>
              <Switch
                :model-value="autoFollowEnabled"
                @update:model-value="(value) => emit('update:auto-follow-enabled', value)"
              />
            </div>

            <!-- Action buttons -->
            <div class="grid grid-cols-2 gap-2 pt-2 border-t">
              <Button v-if="hasLogs" variant="outline" size="sm" :disabled="copyInProgress" @click="copyLogs">
                <Icon v-if="copyInProgress" name="spinner" class="animate-spin mr-2" />
                <Icon v-else :name="logsCopied ? 'lucide:check' : 'lucide:copy'" class="mr-2" />
                {{ logsCopied ? $t('copied') : $t('repo.pipeline.actions.log_copy') }}
              </Button>
              <Button
                v-if="step?.finished !== undefined && hasLogs"
                variant="outline"
                size="sm"
                :disabled="downloadInProgress"
                @click="download"
              >
                <Icon v-if="downloadInProgress" name="spinner" class="animate-spin mr-2" />
                <Icon v-else name="download" class="mr-2" />
                {{ $t('repo.pipeline.actions.log_download') }}
              </Button>
              <Button
                v-if="step?.finished !== undefined && hasLogs && hasPushPermission"
                variant="outline"
                size="sm"
                @click="showDeleteLogsDialog = true"
              >
                <Icon name="trash" class="mr-2" />
                {{ $t('repo.pipeline.actions.log_delete') }}
              </Button>
              <Button v-if="step?.finished === undefined" variant="outline" size="sm" @click="autoScroll = !autoScroll">
                <Icon :name="autoScroll ? 'auto-scroll' : 'auto-scroll-off'" class="mr-2" />
                {{
                  autoScroll
                    ? $t('repo.pipeline.actions.log_auto_scroll_off')
                    : $t('repo.pipeline.actions.log_auto_scroll')
                }}
              </Button>
            </div>
          </div>
        </SheetContent>
      </Sheet>

      <!-- Log area with floating scroll buttons wrapper -->
      <CardContent
        v-show="hasLogs && loadedLogs && (log?.length || 0) > 0"
        class="relative flex-1 min-h-0 pb-4 flex flex-col"
      >
        <!-- Floating search bar -->
        <Transition
          enter-active-class="transition-all duration-200 ease-out"
          enter-from-class="opacity-0 -translate-y-2"
          enter-to-class="opacity-100 translate-y-0"
          leave-active-class="transition-all duration-150 ease-in"
          leave-from-class="opacity-100 translate-y-0"
          leave-to-class="opacity-0 -translate-y-2"
        >
          <div
            v-if="searchOpen"
            class="absolute top-2 left-2 right-2 z-30 flex items-center gap-2 rounded-lg border bg-background/95 backdrop-blur-sm px-3 py-2 shadow-lg"
          >
            <Icon name="lucide:search" class="h-4 w-4 text-muted-foreground shrink-0" />
            <Input
              ref="searchInputRef"
              v-model="searchTerm"
              type="text"
              :placeholder="$t('repo.pipeline.search.placeholder')"
              class="h-8 flex-1 border-0 bg-transparent focus-visible:ring-0 focus-visible:ring-offset-0"
              @keydown.stop
            />
            <span v-if="searchTerm" class="text-xs text-muted-foreground whitespace-nowrap">
              <template v-if="searchMatches.length > 0">
                {{
                  $t('repo.pipeline.search.match_count', {
                    current: currentMatchIndex + 1,
                    total: searchMatches.length,
                  })
                }}
              </template>
              <template v-else>
                {{ $t('repo.pipeline.search.no_results') }}
              </template>
            </span>
            <div v-if="searchTerm && searchMatches.length > 0" class="flex items-center gap-1">
              <TooltipProvider>
                <Tooltip>
                  <TooltipTrigger as-child>
                    <Button variant="ghost" size="icon-sm" @click="goToPreviousMatch">
                      <Icon name="chevron-up" class="h-4 w-4" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>{{ $t('repo.pipeline.search.previous') }}</TooltipContent>
                </Tooltip>
              </TooltipProvider>
              <TooltipProvider>
                <Tooltip>
                  <TooltipTrigger as-child>
                    <Button variant="ghost" size="icon-sm" @click="goToNextMatch">
                      <Icon name="chevron-down" class="h-4 w-4" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>{{ $t('repo.pipeline.search.next') }}</TooltipContent>
                </Tooltip>
              </TooltipProvider>
            </div>
            <TooltipProvider>
              <Tooltip>
                <TooltipTrigger as-child>
                  <Button variant="ghost" size="icon-sm" @click="closeSearch">
                    <Icon name="close" class="h-4 w-4" />
                  </Button>
                </TooltipTrigger>
                <TooltipContent>{{ $t('close') }} {{ $t('repo.pipeline.search.close_hint') }}</TooltipContent>
              </Tooltip>
            </TooltipProvider>
          </div>
        </Transition>

        <!-- Floating scroll buttons (desktop) -->
        <div class="hidden md:flex flex-col gap-2 absolute bottom-4 right-4 z-20">
          <TooltipProvider v-if="!atTop">
            <Tooltip>
              <TooltipTrigger as-child>
                <Button
                  variant="outline"
                  size="icon-sm"
                  class="rounded-full shadow-lg bg-background dark:bg-background"
                  @click="scrollToTop"
                >
                  <Icon name="chevron-up" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>
                {{ $t('repo.pipeline.actions.log_scroll_to_top') }}
              </TooltipContent>
            </Tooltip>
          </TooltipProvider>
          <TooltipProvider v-if="!atBottom">
            <Tooltip>
              <TooltipTrigger as-child>
                <Button
                  variant="outline"
                  size="icon-sm"
                  class="rounded-full shadow-lg bg-background dark:bg-background"
                  @click="scrollToBottom"
                >
                  <Icon name="chevron-down" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>
                {{ $t('repo.pipeline.actions.log_scroll_to_bottom') }}
              </TooltipContent>
            </Tooltip>
          </TooltipProvider>
        </div>

        <!-- Log Truncation Warning Banner -->
        <Transition
          enter-active-class="transition-all duration-200 ease-out"
          enter-from-class="opacity-0 -translate-y-2"
          enter-to-class="opacity-100 translate-y-0"
          leave-active-class="transition-all duration-150 ease-in"
          leave-from-class="opacity-100 translate-y-0"
          leave-to-class="opacity-0 -translate-y-2"
        >
          <Alert
            v-if="isTruncated"
            variant="default"
            class="mx-3 mt-2 max-w-[calc(100%-1.5rem)] border-yellow-500 bg-yellow-50 dark:bg-yellow-950/30"
          >
            <Icon name="alert" class="text-yellow-600 dark:text-yellow-500 shrink-0" />
            <AlertTitle class="text-yellow-800 dark:text-yellow-400">
              {{ $t('repo.pipeline.log_truncated_title') }}
            </AlertTitle>
            <AlertDescription class="text-yellow-700 dark:text-yellow-500 wrap-break-word">
              {{ $t('repo.pipeline.log_truncated_desc', { limit: logLineLimit.toLocaleString() }) }}
            </AlertDescription>
          </Alert>
        </Transition>

        <!-- Scrollable log container -->
        <div ref="logContainerRef" class="flex-1 min-h-0 w-full overflow-auto">
          <div
            class="grid w-full auto-rows-min px-1 md:px-3 pt-2 pb-4 text-xs"
            :class="[
              hideTimestamps
                ? 'grid-cols-[min-content_minmax(0,1fr)]'
                : 'grid-cols-[min-content_minmax(0,1fr)_min-content]',
            ]"
          >
            <div
              v-for="line in log"
              :key="line.index"
              class="grid grid-cols-subgrid font-mono"
              :class="hideTimestamps ? 'col-span-2' : 'col-span-3'"
            >
              <a
                :id="`L${line.number}`"
                :href="`#L${line.number}`"
                class="select-none whitespace-nowrap pl-2 pr-2 md:pr-6 text-right text-muted-foreground"
                :class="{
                  'bg-red-600/40 dark:bg-red-800/50': line.type === 'error',
                  'bg-yellow-600/40 dark:bg-yellow-800/50': line.type === 'warning',
                  'bg-blue-600/30': isSelected(line),
                  underline: isSelected(line),
                }"
              >
                {{ line.number }}
              </a>
              <!-- eslint-disable vue/no-v-html -->
              <span
                class="min-w-0 whitespace-pre-wrap wrap-anywhere align-top"
                :class="{
                  'bg-red-600/40 dark:bg-red-800/50': line.type === 'error',
                  'bg-yellow-600/40 dark:bg-yellow-800/50': line.type === 'warning',
                  'bg-blue-600/30': isSelected(line),
                  'ring-2 ring-yellow-500 ring-offset-1': isCurrentSearchMatch(line),
                }"
                v-html="highlightSearchTerm(line.text || '')"
              />
              <!-- eslint-enable vue/no-v-html -->
              <span
                v-if="!hideTimestamps"
                class="select-none whitespace-nowrap pr-1 text-right text-muted-foreground"
                :class="{
                  'bg-red-600 bg-opacity-40 dark:bg-red-800/50': line.type === 'error',
                  'bg-yellow-600/40 dark:bg-yellow-800/50': line.type === 'warning',
                  'bg-blue-600/30': isSelected(line),
                }"
                :title="formatAbsoluteTime(line.time)"
              >
                {{ formatTime(line.time) }}
              </span>
            </div>
          </div>
        </div>
      </CardContent>

      <CardContent class="m-auto text-xl text-muted-foreground">
        <span v-if="step?.state === 'skipped'">{{ $t('repo.pipeline.actions.canceled') }}</span>
        <div
          v-else-if="!step?.started && currentWorkflow?.autoscaler_provisioning"
          class="flex flex-col items-center gap-3 text-center"
        >
          <template v-if="currentWorkflow?.autoscaler_provisioning_stale">
            <Icon name="lucide:triangle-alert" class="w-10 h-10 text-destructive" />
            <span class="text-lg font-medium text-destructive">{{ $t('repo.pipeline.agent_provisioning_stale') }}</span>
            <span class="text-sm text-muted-foreground">{{ $t('repo.pipeline.agent_provisioning_stale_desc') }}</span>
          </template>
          <template v-else>
            <Icon name="lucide:server" class="w-10 h-10 text-primary animate-pulse" />
            <span class="text-lg font-medium text-foreground">{{ $t('repo.pipeline.agent_provisioning') }}</span>
            <span class="text-sm text-muted-foreground">{{ $t('repo.pipeline.agent_provisioning_desc') }}</span>
          </template>
        </div>
        <span v-else-if="!step?.started">{{ $t('repo.pipeline.step_not_started') }}</span>
        <div v-else-if="!loadedLogs">{{ $t('repo.pipeline.loading') }}</div>

        <div v-else-if="logsDeleted">{{ $t('repo.pipeline.logs_deleted') }}</div>
        <div v-else-if="log?.length === 0 && awaitingLogs" class="flex items-center gap-3">
          <Icon name="spinner" class="w-6 h-6 animate-spin text-primary" />
          <span>{{ $t('repo.pipeline.waiting_for_logs') }}</span>
        </div>
        <div v-else-if="log?.length === 0" class="flex flex-col items-center gap-3 text-center">
          <span>{{ $t('repo.pipeline.no_logs') }}</span>
          <Button variant="outline" size="sm" @click="loadLogs(true)">
            <Icon name="solar:restart-bold" class="mr-2" />
            {{ $t('repo.pipeline.reload_logs') }}
          </Button>
        </div>
      </CardContent>

      <!-- Stderr Error Output Section -->
      <div
        v-if="stderrLines.length > 0"
        data-stderr-section
        class="border-t border-destructive bg-red-50 dark:bg-red-950/20"
      >
        <button
          class="w-full flex items-center justify-between px-3 py-2 hover:bg-red-100 dark:hover:bg-red-900/30 transition-colors"
          @click="stderrExpanded = !stderrExpanded"
        >
          <div class="flex items-center gap-2">
            <Icon name="alert" class="w-4 h-4 text-destructive dark:text-red-500" />
            <span class="text-sm font-semibold text-destructive dark:text-red-500">
              {{ $t('repo.pipeline.actions.stderr_output') }}
            </span>
            <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
            <span class="text-sm font-semibold text-destructive dark:text-red-500">({{ stderrLines.length }})</span>
          </div>
          <Icon
            :name="stderrExpanded ? 'chevron-up' : 'chevron-down'"
            class="w-4 h-4 text-destructive dark:text-red-500"
          />
        </button>
        <div v-show="stderrExpanded" class="relative px-3 pb-2">
          <Button
            variant="ghost"
            size="icon-sm"
            class="absolute top-2 right-5 text-destructive dark:text-red-500 hover:bg-red-100 dark:hover:bg-red-900/30 bg-white dark:bg-black/40"
            @click="copyStderr"
          >
            <Icon v-if="stderrCopied" name="lucide:check" class="w-4 h-4" />
            <Icon v-else name="lucide:copy" class="w-4 h-4" />
          </Button>
          <div class="max-h-96 overflow-y-auto">
            <!-- eslint-disable vue/no-v-html -->
            <pre
              class="text-xs font-mono bg-white dark:bg-black/20 rounded px-2 py-2 pr-10 border border-red-200 dark:border-red-800 text-destructive dark:text-red-500 whitespace-pre-wrap wrap-break-word"
              v-html="highlightSearchTerm(stderrLines.map((e) => e.text).join('\n'))"
            />
            <!-- eslint-enable vue/no-v-html -->
          </div>
        </div>
      </div>

      <div v-if="step?.error" class="text-xs flex w-full items-center gap-2 border-t px-3 py-1.5 text-foreground">
        <span class="border border-destructive rounded-full px-2 py-0.5">{{ step.error }}</span>
      </div>
    </Card>

    <AlertDialog v-model:open="showDeleteLogsDialog">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{{ $t('repo.pipeline.actions.log_delete') }}</AlertDialogTitle>
          <AlertDialogDescription>{{ $t('repo.pipeline.log_delete_confirm') }}</AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>{{ $t('cancel') }}</AlertDialogCancel>
          <AlertDialogAction
            class="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            @click="confirmDeleteLogs"
          >
            {{ $t('repo.pipeline.actions.log_delete') }}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  </component>
</template>

<script lang="ts" setup>
import '~/style/console.css';
import { useStorage } from '@vueuse/core';
import Icon from '~/components/atomic/Icon.vue';
import PipelineStatusIcon from '~/components/repo/pipeline/PipelineStatusIcon.vue';
import { Alert, AlertDescription, AlertTitle } from '~/components/ui/alert';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '~/components/ui/alert-dialog';
import { Button } from '~/components/ui/button';
import { Card, CardContent, CardHeader } from '~/components/ui/card';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '~/components/ui/dropdown-menu';
import { Input } from '~/components/ui/input';
import {
  NumberField,
  NumberFieldContent,
  NumberFieldDecrement,
  NumberFieldIncrement,
  NumberFieldInput,
} from '~/components/ui/number-field';
import { Popover, PopoverContent, PopoverTrigger } from '~/components/ui/popover';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '~/components/ui/select';
import { Sheet, SheetContent, SheetHeader, SheetTitle } from '~/components/ui/sheet';
import { Switch } from '~/components/ui/switch';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '~/components/ui/tooltip';
import useApiClient from '~/compositions/useApiClient';
import { requiredInject } from '~/compositions/useInjectProvide';
import useNotifications from '~/compositions/useNotifications';
import type { Pipeline, PipelineStep, PipelineWorkflow } from '~/lib/api/types';
import { duplicateStepNameIndexes, getWorkflowAttempts } from '~/lib/pipeline-utils';
import { AnsiUp } from 'ansi_up';
import DOMPurify from 'dompurify';
import type { Config as DOMPurifyConfig } from 'dompurify';
import { decode } from 'js-base64';
import { debounce } from 'lodash';
import { computed, nextTick, onBeforeUnmount, onMounted, ref, toRef, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRoute } from 'vue-router';

const props = withDefaults(
  defineProps<{
    pipeline: Pipeline;
    stepId: number;
    previousStepId?: number | null;
    nextStepId?: number | null;
    autoFollowEnabled?: boolean;
    hideTimestamps?: boolean;
    class?: string;
  }>(),
  {
    autoFollowEnabled: false,
    hideTimestamps: false,
    class: '',
  },
);
const emit = defineEmits<{
  (event: 'update:step-id', stepId: number | null): true;
  (event: 'update:auto-follow-enabled', value: boolean): true;
  (event: 'rerunWorkflow', workflowId: number): true;
  (event: 'rerunStep', workflowId: number, stepId: number): true;
}>();
const HTML_TAG_REGEX = /<[^>]*>/g;
const URL_REGEX = /https?:\/\/\S+/g;
const SPECIAL_REGEX_CHARS = /[.*+?^${}()|[\]\\]/g;
const WARNING_PATTERNS_REGEX = /\bwarn(?:ing)?s?\b|\[warn(?:ing)?\]/i;
const SUCCESS_PATTERNS_REGEX =
  /^\s*[v✓✔☑⋅·•›>+⟶→]\s|done\b|success|complete[ds]?\b|installed\b|built\b|packaged\b|updated\b|finished\b|passed\b|ok\b|\bgot\b/i;
const INFO_PATTERNS_REGEX = /^\s*[i>+\-*•·]\s|^\s*\d+\s*(?:pkg|packages?|deps?)/i;
const ERROR_PATTERNS_REGEX =
  /\berror\b|\bfailed\b|\bfailure\b|\bfatal\b|\bcritical\b|\bexception\b|\bpanic\b|\baborted?\b/i;

interface LogLine {
  index: number;
  number: number;
  text?: string;
  time?: number;
  type: 'error' | 'warning' | null;
}

const notifications = useNotifications();
const i18n = useI18n();
const pipeline = toRef(props, 'pipeline');
const stepId = toRef(props, 'stepId');
const repo = requiredInject('repo');
const repoPermissions = requiredInject('repo-permissions');
const apiClient = useApiClient();
const route = useRoute();

const loadedStepSlug = ref<string>();
const stepSlug = computed(() => `${repo?.value.owner} - ${repo?.value.name} - ${pipeline.value.id} - ${stepId.value}`);
const step = computed(() => pipeline.value && findStep(pipeline.value.workflows || [], stepId.value));
const currentWorkflow = computed(() => {
  if (!pipeline.value?.workflows) return null;
  return pipeline.value.workflows.find((wf) => wf.children?.some((s) => s.pid === stepId.value)) ?? null;
});
const isStepCompleted = computed(() => {
  const s = step.value;
  return s != null && s.state !== 'pending' && s.state !== 'running';
});
const workflowAttempts = computed(() => {
  const wf = currentWorkflow.value;
  if (wf == null || pipeline.value?.workflows == null) return [];
  return getWorkflowAttempts(pipeline.value.workflows, wf);
});
const hasMultipleAttempts = computed(() => workflowAttempts.value.length > 1);

const stepNameSuffix = computed(() => {
  const wf = currentWorkflow.value;
  const s = step.value;
  if (wf == null || s == null) return '';
  const index = duplicateStepNameIndexes(wf.children).get(s.pid);
  return index ? ` (${index})` : '';
});

function switchToAttempt(attempt: PipelineWorkflow) {
  // Navigate to the first step of the selected attempt
  if (attempt.children.length > 0) {
    emit('update:step-id', attempt.children[0].pid);
  }
}

const stream = ref<EventSource>();
const log = ref<LogLine[]>();
// Guards the debounced flushLogs from touching detached refs after the
// component has been torn down (the timer can fire up to 500ms post-unmount).
let isUnmounted = false;
const logContainerRef = ref<HTMLElement | null>(null);
const componentRef = ref<HTMLElement | null>(null);
const logsDeleted = computed(() => pipeline.value.logs_deleted);

const loadedLogs = computed(() => !!log.value);
const hasLogs = computed(
  () =>
    // we do not have logs for skipped steps
    repo?.value && pipeline.value && step.value && step.value.state !== 'skipped',
);
// A live step keeps producing output, so an empty log view is expected rather
// than a sign that there is nothing to show.
const isStepLive = computed(() => step.value?.state === 'running' || step.value?.state === 'pending');
// Backoff used to re-read the store while logs are still being persisted: a
// step can be reported as finished a moment before its last batch lands.
const LOG_RETRY_DELAYS_MS = [1000, 2000, 4000, 8000];
let logRetryTimer: ReturnType<typeof setTimeout> | undefined;
const logRetryPending = ref(false);
// Distinguishes "there is nothing to show yet" from "there is nothing to show".
const awaitingLogs = computed(() => isStepLive.value || logRetryPending.value);
const autoScroll = useStorage('crow:log-auto-scroll', false);
const downloadInProgress = ref(false);
const copyInProgress = ref(false);
const logsCopied = ref(false);
let logsCopiedTimer: ReturnType<typeof setTimeout> | undefined;
const ansiUp = ref(new AnsiUp());
ansiUp.value.use_classes = true;
const logBuffer = ref<LogLine[]>([]);

// Configurable log line limit - user can adjust in settings
const logLineLimit = useStorage('crow:log-line-limit', 10000);
const logLineLimitTemp = ref(logLineLimit.value);
const hasPushPermission = computed(() => repoPermissions?.value?.push);

// Track if logs have been truncated (for warning banner)
const isTruncated = ref(false);

const mobileSettingsOpen = ref(false);
const atTop = ref(true);
const atBottom = ref(false);
const isScrollable = ref(false);
const isFullscreen = ref(false);
const stderrExpanded = ref(true);
const stderrCopied = ref(false);

// Search state
const searchOpen = ref(false);
const searchTerm = ref('');
const currentMatchIndex = ref(0);
const searchInputRef = ref<HTMLInputElement | null>(null);

// Extract stderr lines (error output) from logs
const stderrLines = computed(() => {
  if (!log.value || log.value.length === 0 || step.value?.exit_code === 0) {
    return [];
  }

  // Filter lines that are marked as stderr (type === 'error')
  const errors = log.value
    .filter((line) => line.type === 'error')
    .map((line) => ({
      line: line.number,
      text: (line.text?.replace(HTML_TAG_REGEX, '') || '').trim(), // Strip HTML
    }))
    .filter((error) => error.text.length > 0); // Remove empty lines

  return errors;
});

// Search: find all matching line indices
const searchMatches = computed(() => {
  if (!searchTerm.value || searchTerm.value.length === 0) {
    return [];
  }

  const term = searchTerm.value.toLowerCase();
  const matches: { type: 'log' | 'stderr'; index: number; lineNumber: number }[] = [];

  // Search main logs
  if (log.value) {
    log.value.forEach((line, idx) => {
      // Strip HTML tags and check for match
      const plainText = (line.text?.replace(HTML_TAG_REGEX, '') || '').toLowerCase();
      if (plainText.includes(term)) {
        matches.push({ type: 'log', index: idx, lineNumber: line.number });
      }
    });
  }

  // Search stderr lines (only if stderr section is expanded)
  if (stderrExpanded.value && stderrLines.value.length > 0) {
    stderrLines.value.forEach((line, idx) => {
      if (line.text.toLowerCase().includes(term)) {
        matches.push({ type: 'stderr', index: idx, lineNumber: line.line });
      }
    });
  }

  return matches;
});

// Current match for display
const currentMatch = computed(() => {
  if (searchMatches.value.length === 0) return null;
  const idx = Math.max(0, Math.min(currentMatchIndex.value, searchMatches.value.length - 1));
  return searchMatches.value[idx];
});

// Allowlist for the log HTML we render via v-html. ansi_to_html output plus our
// URL-linking step is safe by construction, but we sanitize as insurance.
const LOG_SANITIZE_CONFIG: DOMPurifyConfig = {
  ALLOWED_TAGS: ['span', 'a', 'mark'],
  ALLOWED_ATTR: ['href', 'class', 'target', 'rel', 'style'],
};

/**
 * Wrap search-term matches in <mark> elements, operating on text nodes only so
 * we never inject markup inside tags or attribute values (e.g. searching for
 * "href" or a class name). The result is sanitized before being handed to
 * v-html as defense-in-depth.
 */
function highlightSearchTerm(text: string): string {
  if (!searchTerm.value || searchTerm.value.length === 0) {
    return DOMPurify.sanitize(text, LOG_SANITIZE_CONFIG);
  }

  const template = document.createElement('template');
  template.innerHTML = text;

  const regex = new RegExp(escapeRegExp(searchTerm.value), 'gi');
  const walker = document.createTreeWalker(template.content, NodeFilter.SHOW_TEXT);
  const textNodes: Text[] = [];
  let node = walker.nextNode();
  while (node) {
    textNodes.push(node as Text);
    node = walker.nextNode();
  }

  textNodes.forEach((textNode) => {
    const value = textNode.nodeValue ?? '';
    regex.lastIndex = 0;
    if (!regex.test(value)) {
      return;
    }

    const fragment = document.createDocumentFragment();
    let lastIndex = 0;
    regex.lastIndex = 0;
    let match = regex.exec(value);
    while (match) {
      if (match.index > lastIndex) {
        fragment.appendChild(document.createTextNode(value.slice(lastIndex, match.index)));
      }
      const mark = document.createElement('mark');
      mark.className = 'bg-yellow-300 dark:bg-yellow-700 text-inherit';
      mark.textContent = match[0];
      fragment.appendChild(mark);
      lastIndex = match.index + match[0].length;
      // Guard against zero-length matches to avoid an infinite loop.
      if (match[0].length === 0) {
        regex.lastIndex += 1;
      }
      match = regex.exec(value);
    }
    if (lastIndex < value.length) {
      fragment.appendChild(document.createTextNode(value.slice(lastIndex)));
    }
    textNode.parentNode?.replaceChild(fragment, textNode);
  });

  return DOMPurify.sanitize(template.innerHTML, LOG_SANITIZE_CONFIG);
}

// Helper to escape special regex characters
function escapeRegExp(string: string): string {
  return string.replace(SPECIAL_REGEX_CHARS, '\\$&');
}

// Check if a log line is the current search match
function isCurrentSearchMatch(line: LogLine): boolean {
  if (!currentMatch.value || currentMatch.value.type !== 'log') return false;
  return currentMatch.value.lineNumber === line.number;
}

const logTheme = useStorage('crow:log-theme', 'auto');
const logThemeClass = computed(() => {
  const theme = logTheme.value;
  if (theme === 'light') {
    return 'log-theme-light';
  }
  if (theme === 'dark') {
    return 'log-theme-dark';
  }
  // 'auto' - no additional class, follows system theme
  return '';
});

const logThemeOptions = computed(() => [
  { value: 'auto', text: i18n.t('user.settings.general.log_theme_system') },
  { value: 'light', text: i18n.t('user.settings.general.log_theme_light') },
  { value: 'dark', text: i18n.t('user.settings.general.log_theme_dark') },
]);

const pipelineSplitRatio = useStorage('crow:pipeline-sidebar-size', 30);
const pipelineSplitRatioTemp = ref(pipelineSplitRatio.value);
const settingsPopoverOpen = ref(false);

// Watch for popover opening to sync temp values
watch(settingsPopoverOpen, (isOpen) => {
  if (isOpen) {
    // When opening, sync temp values from storage
    pipelineSplitRatioTemp.value = pipelineSplitRatio.value;
    logLineLimitTemp.value = logLineLimit.value;
  } else {
    // When closing, save temp values to storage
    pipelineSplitRatio.value = pipelineSplitRatioTemp.value;
    logLineLimit.value = logLineLimitTemp.value;
  }
});

function isSelected(line: LogLine): boolean {
  return route.hash === `#L${line.number}`;
}

function formatTime(time?: number): string {
  if (time === undefined) return '';
  const hours = Math.floor(time / 3600);
  const minutes = Math.floor((time % 3600) / 60);
  const seconds = Math.floor(time % 60);
  let result = '';
  if (hours > 0) result += `${hours}h`;
  if (hours > 0 || minutes > 0) result += `${minutes}m`;
  return `${result}${seconds}s`;
}

function formatAbsoluteTime(time?: number): string {
  const started = step.value?.started;
  if (time === undefined || started === undefined) return '';
  return new Date((started + time) * 1000).toLocaleTimeString(undefined, {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  });
}

function processText(text: string): string {
  let txt = ansiUp.value.ansi_to_html(`${decode(text)}\n`);
  txt = txt.replace(
    URL_REGEX,
    (url) => `<a href="${url}" target="_blank" rel="noopener noreferrer" class="underline">${url}</a>`,
  );
  return txt;
}

/**
 * Classify a log line based on its content to determine highlighting.
 * Many CLI tools output informational messages to stderr, so we analyze
 * the actual content rather than relying solely on the stream type.
 */
function classifyLogLine(text: string, isStderr: boolean): 'error' | 'warning' | null {
  // Warning patterns - check first since warnings shouldn't be treated as errors
  if (WARNING_PATTERNS_REGEX.test(text)) {
    return 'warning';
  }

  // Success/info indicators - these should never be highlighted as errors
  // Matches checkmarks (v, ✓, ✔), info symbols (i), progress indicators from common CLI tools
  if (SUCCESS_PATTERNS_REGEX.test(text)) {
    return null;
  }

  // Skip highlighting for info prefixes (i for info) and common progress indicators
  // These are common in package managers and build tools
  if (INFO_PATTERNS_REGEX.test(text)) {
    return null;
  }

  // Error patterns - only highlight if it looks like an actual error
  if (isStderr && ERROR_PATTERNS_REGEX.test(text)) {
    return 'error';
  }

  // For stderr lines without explicit error patterns, don't highlight
  // This prevents false positives from tools that use stderr for progress info
  return null;
}

function writeLog(line: Partial<LogLine> & { logType?: number }) {
  // Map backend LogEntryType to frontend type
  // 0 = stdout (null), 1 = stderr, 2 = exit code, 3 = metadata, 4 = progress
  const isStderr = line.logType === 1;
  const rawText = line.text ?? '';

  // Use content-based classification instead of just stderr detection
  const type = classifyLogLine(rawText, isStderr);

  logBuffer.value.push({
    index: line.index ?? 0,
    number: (line.index ?? 0) + 1,
    text: processText(rawText),
    time: line.time ?? 0,
    type,
  });
}

function getScrollableElement(): HTMLElement | null {
  const element = logContainerRef.value;
  if (!element) return null;

  // Check if log container is scrollable (has overflow)
  if (element.scrollHeight > element.clientHeight) {
    return element;
  }

  // On mobile, the scroll might be on a parent ScrollArea viewport
  // Find the closest scrollable parent
  let parent = element.parentElement;
  while (parent) {
    const style = getComputedStyle(parent);
    const overflowY = style.overflowY;
    if ((overflowY === 'auto' || overflowY === 'scroll') && parent.scrollHeight > parent.clientHeight) {
      return parent;
    }
    // Check for data-radix-scroll-area-viewport (ScrollArea's viewport)
    if (parent.hasAttribute('data-radix-scroll-area-viewport')) {
      return parent;
    }
    parent = parent.parentElement;
  }

  // Fallback to document scrolling element
  return (document.scrollingElement as HTMLElement) || document.documentElement;
}

function scrollToBottom() {
  nextTick(() => {
    const element = getScrollableElement();
    if (element) {
      element.scrollTo({ top: element.scrollHeight, behavior: 'smooth' });
    }
  });
}

function scrollToTop() {
  nextTick(() => {
    const element = getScrollableElement();
    if (element) {
      element.scrollTo({ top: 0, behavior: 'smooth' });
    }
  });
}

const flushLogs = debounce((scroll: boolean) => {
  // A pending debounce can still fire after unmount; bail out so we don't
  // mutate detached refs or touch a torn-down DOM.
  if (isUnmounted) {
    return;
  }
  const limit = logLineLimit.value;

  // Track pre-truncation total for accurate detection
  const preTruncationTotal = logBuffer.value.length + (log.value?.length ?? 0);

  let buffer = logBuffer.value.slice(-limit);
  logBuffer.value = [];

  if (buffer.length === 0) {
    if (!log.value) {
      log.value = [];
    }
    return;
  }

  // append old logs lines
  if (buffer.length < limit && log.value) {
    buffer = [...log.value.slice(-(limit - buffer.length)), ...buffer];
  }

  // deduplicate lines by index: an SSE reconnect can replay already-seen
  // lines, and duplicate keys make Vue mis-patch a row's grid cells, shifting
  // the whole log layout. Keep the last occurrence of each index.
  const seenIndex = new Map<number, number>();
  buffer.forEach((line, i) => seenIndex.set(line.index, i));
  buffer = buffer.filter((line, i) => seenIndex.get(line.index) === i);

  // deduplicate repeating times
  buffer = buffer.reduce(
    (acc, line) => ({
      lastTime: line.time ?? 0,
      lines: [
        ...acc.lines,
        {
          ...line,
          time: acc.lastTime === line.time ? undefined : line.time,
        },
      ],
    }),
    { lastTime: -1, lines: [] as LogLine[] },
  ).lines;

  // Set truncation flag if we had more lines than the limit allows
  // This means some lines were discarded during truncation
  if (preTruncationTotal > limit) {
    isTruncated.value = true;
  }

  log.value = buffer;

  if (route.hash.length > 0) {
    nextTick(() => document.getElementById(route.hash.substring(1))?.scrollIntoView());
  } else if (scroll && autoScroll.value) {
    scrollToBottom();
  }
}, 500);

async function download() {
  if (!repo?.value || !pipeline.value || !step.value) {
    throw new Error('The repository, pipeline or step was undefined');
  }
  let logs;
  try {
    downloadInProgress.value = true;
    logs = await apiClient.getLogs(repo.value.id, pipeline.value.number, step.value.id);
  } catch {
    notifications.notify({
      title: i18n.t('notify_title_error'),
      text: i18n.t('repo.pipeline.log_download_error'),
      type: 'error',
      duration: 9000,
    });
    return;
  } finally {
    downloadInProgress.value = false;
  }
  const fileURL = window.URL.createObjectURL(
    new Blob([logs.map((line) => decode(line.data ?? '')).join('\n')], {
      type: 'text/plain',
    }),
  );
  const fileLink = document.createElement('a');

  fileLink.href = fileURL;
  fileLink.setAttribute(
    'download',
    `${repo.value.owner}-${repo.value.name}-${pipeline.value.number}-${step.value.name}.log`,
  );
  document.body.appendChild(fileLink);

  fileLink.click();
  document.body.removeChild(fileLink);
  window.URL.revokeObjectURL(fileURL);
}

async function copyLogs() {
  if (!repo?.value || !pipeline.value || !step.value) {
    throw new Error('The repository, pipeline or step was undefined');
  }

  try {
    copyInProgress.value = true;
    const logs = await apiClient.getLogs(repo.value.id, pipeline.value.number, step.value.id);
    await navigator.clipboard.writeText(logs.map((line) => decode(line.data ?? '')).join('\n'));
    logsCopied.value = true;
    if (logsCopiedTimer) clearTimeout(logsCopiedTimer);
    logsCopiedTimer = setTimeout(() => {
      logsCopied.value = false;
    }, 2000);
  } catch {
    notifications.notify({
      title: i18n.t('notify_title_error'),
      text: i18n.t('repo.pipeline.log_copy_error'),
      type: 'error',
      duration: 9000,
    });
  } finally {
    copyInProgress.value = false;
  }
}

function clearLogRetry() {
  if (logRetryTimer) {
    clearTimeout(logRetryTimer);
    logRetryTimer = undefined;
  }
  logRetryPending.value = false;
}

/** True once at least one line is known, either rendered or still buffered. */
function hasReceivedLines(): boolean {
  return (log.value?.length ?? 0) > 0 || logBuffer.value.length > 0;
}

/** Read the persisted logs of the current step and return how many lines were found. */
async function fetchPersistedLogs(): Promise<number> {
  if (!repo?.value || !pipeline.value || !step.value) {
    return 0;
  }
  const logs = await apiClient.getLogs(repo.value.id, pipeline.value.number, step.value.id);
  logs?.forEach((line) => writeLog({ index: line.line, text: line.data, time: line.time, logType: line.type }));
  flushLogs(false);
  return logs?.length ?? 0;
}

/**
 * Re-read the store while the view is still empty. Logs are persisted
 * asynchronously, so opening a step right after it started (or right after it
 * finished) can hit a window where the store is still empty and the in-memory
 * stream has nothing to replay yet. Instead of claiming there are no logs we
 * retry with a backoff and keep showing a waiting state.
 */
function scheduleLogRetry(slug: string, attempt = 0) {
  const delay = LOG_RETRY_DELAYS_MS[attempt];
  if (delay === undefined) {
    logRetryPending.value = false;
    return;
  }

  logRetryPending.value = true;
  logRetryTimer = setTimeout(async () => {
    logRetryTimer = undefined;
    // Bail out if we were torn down, switched steps, or logs arrived meanwhile.
    if (isUnmounted || loadedStepSlug.value !== slug || hasReceivedLines()) {
      logRetryPending.value = false;
      return;
    }

    try {
      if (await fetchPersistedLogs()) {
        logRetryPending.value = false;
        return;
      }
    } catch {
      // keep retrying, the step may simply not have any logs stored yet
    }

    scheduleLogRetry(slug, attempt + 1);
  }, delay);
}

async function loadLogs(force = false) {
  const isSameStep = loadedStepSlug.value === stepSlug.value;
  if (isSameStep && !force) {
    return;
  }

  clearLogRetry();
  // Keep what we already have when reloading the same step (e.g. after it
  // finished) so the view does not flash back to a loading state.
  if (!isSameStep) {
    log.value = undefined;
  }
  logBuffer.value = [];
  isTruncated.value = false; // Reset truncation state for new logs
  ansiUp.value = new AnsiUp();
  ansiUp.value.use_classes = true;

  stream.value?.close();
  stream.value = undefined;

  if (!hasLogs.value || !step.value) {
    return;
  }

  loadedStepSlug.value = stepSlug.value;
  const slug = stepSlug.value;

  if (step.value.state !== 'running' && step.value.state !== 'pending') {
    let lines = 0;
    try {
      lines = await fetchPersistedLogs();
    } catch {
      // fall through to the retry below
    }
    if (lines === 0 && !hasReceivedLines()) {
      scheduleLogRetry(slug);
    }
  } else {
    stream.value = apiClient.streamLogs(repo.value.id, pipeline.value.number, step.value.id, (line) => {
      clearLogRetry();
      writeLog({ index: line.line, text: line.data, time: line.time, logType: line.type });
      flushLogs(true);
    });
    // The stream only replays what the server still holds in memory, which is
    // empty after a server restart. Fall back to the store if it stays quiet.
    if (step.value.started) {
      scheduleLogRetry(slug);
    }
  }
}

const showDeleteLogsDialog = ref(false);

async function confirmDeleteLogs() {
  if (!repo?.value || !pipeline.value || !step.value) {
    throw new Error('The repository, pipeline or step was undefined');
  }

  try {
    await apiClient.deleteLogs(repo.value.id, pipeline.value.number, step.value.id);
    log.value = [];
  } catch {
    notifications.notify({
      title: i18n.t('notify_title_error'),
      text: i18n.t('repo.pipeline.log_delete_error'),
      type: 'error',
      duration: 9000,
    });
  } finally {
    showDeleteLogsDialog.value = false;
  }
}

async function copyStderr() {
  const text = stderrLines.value.map((e) => e.text).join('\n');
  try {
    await navigator.clipboard.writeText(text);
    stderrCopied.value = true;
    setTimeout(() => {
      stderrCopied.value = false;
    }, 2000);
  } catch {
    notifications.notify({
      title: i18n.t('notify_title_error'),
      text: 'Failed to copy to clipboard',
      type: 'error',
      duration: 3000,
    });
  }
}

function findStep(workflows: PipelineWorkflow[], pid: number): PipelineStep | undefined {
  return workflows.reduce(
    (prev, workflow) => {
      const result = workflow.children.reduce(
        (prevChild, step) => {
          if (step.pid === pid) {
            return step;
          }

          return prevChild;
        },
        undefined as PipelineStep | undefined,
      );
      if (result) {
        return result;
      }

      return prev;
    },
    undefined as PipelineStep | undefined,
  );
}

function updateScrollState() {
  const el = logContainerRef.value;
  if (!el) {
    return;
  }
  isScrollable.value = el.scrollHeight > el.clientHeight;
  atTop.value = el.scrollTop === 0;
  atBottom.value = el.scrollTop + el.clientHeight >= el.scrollHeight - 5; // more forgiving for rounding
}

// Debounced scroll handler for better performance
const debouncedUpdateScrollState = debounce(updateScrollState, 50);

onMounted(() => {
  nextTick(() => {
    const el = logContainerRef.value;
    if (el) {
      // Use passive listener to improve scroll performance
      el.addEventListener('scroll', debouncedUpdateScrollState, { passive: true });
      updateScrollState();
    }
  });
});

watch(log, () => {
  nextTick(() => {
    updateScrollState();
    setTimeout(updateScrollState, 50); // ensure update after rendering
  });
});

onBeforeUnmount(() => {
  const el = logContainerRef.value;
  if (el) {
    el.removeEventListener('scroll', debouncedUpdateScrollState);
  }
  if (logsCopiedTimer) {
    clearTimeout(logsCopiedTimer);
  }
});

// Handle keyboard shortcuts
function handleKeydown(event: KeyboardEvent) {
  // Ctrl/Cmd+F to open search — only intercept when the log pane has focus
  // or the log search is already open, otherwise let the browser handle it
  if ((event.metaKey || event.ctrlKey) && event.key === 'f') {
    const hasFocus = searchOpen.value || (componentRef.value && componentRef.value.contains(document.activeElement));
    if (!hasFocus) return;
    event.preventDefault();
    openSearch();
    return;
  }

  // ESC to close search or exit fullscreen
  if (event.key === 'Escape') {
    if (searchOpen.value) {
      closeSearch();
      return;
    }
    if (isFullscreen.value) {
      isFullscreen.value = false;
      return;
    }
  }

  // Enter/Shift+Enter to navigate matches (only when search input is focused)
  if (event.key === 'Enter' && searchOpen.value) {
    event.preventDefault();
    if (event.shiftKey) {
      goToPreviousMatch();
    } else {
      goToNextMatch();
    }
  }
}

// Search functions
function openSearch() {
  searchOpen.value = true;
  nextTick(() => {
    searchInputRef.value?.focus();
    searchInputRef.value?.select();
  });
}

function closeSearch() {
  searchOpen.value = false;
  searchTerm.value = '';
  currentMatchIndex.value = 0;
}

function goToNextMatch() {
  if (searchMatches.value.length === 0) return;
  currentMatchIndex.value = (currentMatchIndex.value + 1) % searchMatches.value.length;
  scrollToCurrentMatch();
}

function goToPreviousMatch() {
  if (searchMatches.value.length === 0) return;
  currentMatchIndex.value = (currentMatchIndex.value - 1 + searchMatches.value.length) % searchMatches.value.length;
  scrollToCurrentMatch();
}

function scrollToCurrentMatch() {
  const match = currentMatch.value;
  if (!match) return;

  if (match.type === 'log') {
    // Scroll to log line
    const lineElement = document.getElementById(`L${match.lineNumber}`);
    if (lineElement) {
      lineElement.scrollIntoView({ behavior: 'smooth', block: 'center' });
    }
  } else if (match.type === 'stderr') {
    // Ensure stderr is expanded and scroll into view
    stderrExpanded.value = true;
    nextTick(() => {
      const stderrSection = document.querySelector('[data-stderr-section]');
      if (stderrSection) {
        stderrSection.scrollIntoView({ behavior: 'smooth', block: 'center' });
      }
    });
  }
}

onMounted(async () => {
  await loadLogs();
  window.addEventListener('keydown', handleKeydown);
});

onBeforeUnmount(() => {
  isUnmounted = true;
  // Cancel pending debounced work so it can't fire against detached refs.
  flushLogs.cancel();
  debouncedUpdateScrollState.cancel();
  clearLogRetry();
  stream.value?.close();
  window.removeEventListener('keydown', handleKeydown);
});

watch(stepSlug, async () => {
  await loadLogs();
});

watch(step, async (newStep, oldStep) => {
  if (oldStep?.name === newStep?.name) {
    if (oldStep?.finished !== newStep?.finished && autoScroll.value) {
      scrollToBottom();
    }

    if (oldStep?.state !== newStep?.state) {
      // Force a reload: the step slug does not change with the state, and a
      // step that just finished needs its logs read back from the store since
      // the stream is closed at that point.
      await loadLogs(true);
    }
  }
});
</script>
