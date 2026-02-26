<script lang="ts">
	import PageContainer from '../_components/page-container/page-container.svelte';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { Separator } from '$lib/components/ui/separator/index.js';
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import * as Alert from '$lib/components/ui/alert/index.js';
	import ListIcon from '@lucide/svelte/icons/list';
	import BookmarkIcon from '@lucide/svelte/icons/bookmark';
	import LogFilters from './_components/log-filters.svelte';
	import LogHistogram from './_components/log-histogram.svelte';
	import LogList from './_components/log-list.svelte';
	import {
		createQueryLogs,
		createGetLogHistogram,
		createGetLogServices,
		queryLogs
	} from '$lib/api/endpoints/logs/logs';
	import { getSandboxJob, getSandboxJobLogs } from '$lib/api/endpoints/sandbox/sandbox';
	import {
		createRestoreJob,
		getRestoreJob,
		parseRestoreRequired,
		type RestoreJob,
		type RestoreRequiredInfo
	} from '$lib/archive/restore';
	import {
		getLogAutoResolvePreview,
		runLogAutoResolve,
		RemediationAPIError,
		type AutoResolvePreview
	} from '$lib/remediation/api';
	import { sessionStore } from '$lib/stores/session';
	import type { LogRecord, HistogramBucket, SandboxJob, SandboxJobLog } from '$lib/api/model';

	type LogLevel = 'debug' | 'info' | 'warn' | 'error' | 'fatal';
	type AutoResolveActivityLevel = 'info' | 'success' | 'warning' | 'error';

	type AutoResolveActivity = {
		seq: number;
		createdAt: string;
		title: string;
		detail?: string;
		level: AutoResolveActivityLevel;
		source: 'opencode' | 'sandbox';
	};

	// Filter UI state (bound to filter components)
	let searchQuery = $state('');
	let selectedService = $state<string | undefined>(undefined);
	let selectedLevel = $state<LogLevel | undefined>(undefined);
	let selectedTimeRange = $state('1h');
	let chartVisible = $state(true);
	let isLive = $state(false);

	// Pagination state
	let extraLogs = $state<LogRecord[]>([]);
	let nextCursor = $state<string | undefined>(undefined);
	let isLoadingMore = $state(false);
	let restoreRequired = $state<RestoreRequiredInfo | null>(null);
	let restoreJob = $state<RestoreJob | null>(null);
	let isStartingRestore = $state(false);
	let isPollingRestore = $state(false);
	let restorePollingJobID = $state<string | undefined>(undefined);
	let restoreError = $state<string | undefined>(undefined);
	let autoResolveDialogOpen = $state(false);
	let autoResolvePreview = $state<AutoResolvePreview | null>(null);
	let autoResolveMissingService = $state<string | undefined>(undefined);
	let autoResolveBusyLogID = $state<string | undefined>(undefined);
	let autoResolveError = $state<string | undefined>(undefined);
	let autoResolveRunLoading = $state(false);
	let autoResolvePolling = $state(false);
	let autoResolveJob = $state<SandboxJob | null>(null);
	let autoResolveJobLogs = $state<SandboxJobLog[]>([]);
	let autoResolveJobLogsCursor = $state(0);

	// Committed search — only updates on Enter, not on every keystroke
	let committedSearch = $state('');

	const orgId = $derived($sessionStore?.active_organization?.id ?? '');

	// Time range durations in milliseconds
	const timeRangeMs: Record<string, number> = {
		'15m': 15 * 60 * 1000,
		'30m': 30 * 60 * 1000,
		'1h': 60 * 60 * 1000,
		'3h': 3 * 60 * 60 * 1000,
		'6h': 6 * 60 * 60 * 1000,
		'12h': 12 * 60 * 60 * 1000,
		'24h': 24 * 60 * 60 * 1000,
		'7d': 7 * 24 * 60 * 60 * 1000,
		'14d': 14 * 24 * 60 * 60 * 1000,
		'30d': 30 * 24 * 60 * 60 * 1000
	};

	// Stable query time range — recomputed only on explicit user actions
	function computeTimeRange(range: string): { start: string; end: string } {
		const now = new Date();
		const ms = timeRangeMs[range] ?? 3600000;
		return {
			start: new Date(now.getTime() - ms).toISOString(),
			end: now.toISOString()
		};
	}

	const initialRange = computeTimeRange('1h');
	let queryStart = $state(initialRange.start);
	let queryEnd = $state(initialRange.end);

	function refreshQueries() {
		const range = computeTimeRange(selectedTimeRange);
		queryStart = range.start;
		queryEnd = range.end;
		extraLogs = [];
		nextCursor = undefined;
		restoreError = undefined;
	}

	// Re-run when dropdown filters change
	$effect(() => {
		selectedTimeRange;
		selectedService;
		selectedLevel;
		refreshQueries();
	});

	function handleSearch() {
		committedSearch = searchQuery;
		refreshQueries();
	}

	// API queries using orval-generated hooks
	// tanstack/svelte-query v6 returns proxy objects, NOT Svelte stores.
	// Access .data / .isPending directly — do NOT use $ prefix.
	const logsQuery = createQueryLogs(
		() => orgId,
		() => ({
			start: queryStart,
			end: queryEnd,
			limit: 100,
			search: committedSearch || undefined,
			service: selectedService || undefined,
			severity: selectedLevel || undefined
		})
	);

	const histogramQuery = createGetLogHistogram(
		() => orgId,
		() => ({
			start: queryStart,
			end: queryEnd,
			search: committedSearch || undefined,
			service: selectedService || undefined,
			severity: selectedLevel || undefined
		})
	);

	const servicesQuery = createGetLogServices(() => orgId);

	// Extract data from query results (direct property access, no $ prefix)
	const firstPageLogs = $derived.by((): LogRecord[] => {
		const resp = logsQuery.data;
		if (resp && resp.status === 200) {
			return resp.data.logs ?? [];
		}
		return [];
	});

	// Track next_cursor from the initial query
	$effect(() => {
		const resp = logsQuery.data;
		if (resp && resp.status === 200 && extraLogs.length === 0) {
			nextCursor = resp.data.next_cursor;
		}
	});

	$effect(() => {
		const resp = logsQuery.data;
		if (!resp) {
			return;
		}
		if (resp.status === 409) {
			const parsed = parseRestoreRequired(resp.data);
			restoreRequired = parsed;
			if (parsed?.jobId && (parsed.jobState === 'queued' || parsed.jobState === 'running')) {
				void pollRestoreJob(parsed.jobId);
			}
			return;
		}
		if (resp.status === 200) {
			restoreRequired = null;
		}
	});

	// Combined logs: first page + extra pages from "load more"
	const allLogs = $derived([...firstPageLogs, ...extraLogs]);
	const hasMore = $derived(!!nextCursor);

	const histogramData = $derived.by((): HistogramBucket[] => {
		const resp = histogramQuery.data;
		if (resp && resp.status === 200) {
			return resp.data.buckets ?? [];
		}
		return [];
	});

	const services = $derived.by((): string[] => {
		const resp = servicesQuery.data;
		if (resp && resp.status === 200) {
			return resp.data.services ?? [];
		}
		return [];
	});

	// Load more logs (cursor-based pagination)
	async function loadMore() {
		if (!nextCursor || isLoadingMore) return;
		isLoadingMore = true;
		try {
			const resp = await queryLogs(orgId, {
				start: queryStart,
				end: queryEnd,
				limit: 100,
				cursor: nextCursor,
				search: committedSearch || undefined,
				service: selectedService || undefined,
				severity: selectedLevel || undefined
			});
			if (resp.status === 200) {
				extraLogs = [...extraLogs, ...(resp.data.logs ?? [])];
				nextCursor = resp.data.next_cursor;
			}
		} finally {
			isLoadingMore = false;
		}
	}

	async function startRestore() {
		if (!restoreRequired || !orgId || isStartingRestore || isPollingRestore) return;
		const confirmed = window.confirm(
			`Restore archived ${restoreRequired.signal} for ${restoreRequired.startDay} to ${restoreRequired.endDay}?`
		);
		if (!confirmed) return;

		isStartingRestore = true;
		restoreError = undefined;
		try {
			restoreJob = await createRestoreJob(
				orgId,
				restoreRequired.signal,
				restoreRequired.startDay,
				restoreRequired.endDay
			);
			await pollRestoreJob(restoreJob.id);
		} catch (error) {
			restoreError = error instanceof Error ? error.message : 'failed to start restore job';
		} finally {
			isStartingRestore = false;
		}
	}

	async function pollRestoreJob(jobID: string) {
		if (!orgId || !jobID || restorePollingJobID === jobID) return;

		restorePollingJobID = jobID;
		isPollingRestore = true;
		restoreError = undefined;
		try {
			for (let attempt = 0; attempt < 120; attempt++) {
				restoreJob = await getRestoreJob(orgId, jobID);

				if (restoreJob.state === 'completed') {
					restoreRequired = null;
					refreshQueries();
					return;
				}
				if (restoreJob.state === 'failed') {
					restoreError = restoreJob.error || 'restore job failed';
					return;
				}

				await new Promise((resolve) => setTimeout(resolve, 3000));
			}

			restoreError = 'restore job is still running';
		} catch (error) {
			restoreError = error instanceof Error ? error.message : 'failed to poll restore status';
		} finally {
			isPollingRestore = false;
			restorePollingJobID = undefined;
		}
	}

	function saveView() {
		console.log('Save view clicked');
	}

	function formatSandboxState(state: string): string {
		return state
			.split('_')
			.map((part) => part.charAt(0).toUpperCase() + part.slice(1))
			.join(' ');
	}

	function truncateText(text: string, max = 220): string {
		if (text.length <= max) return text;
		return `${text.slice(0, max - 1)}…`;
	}

	function compactWhitespace(text: string): string {
		return text.replace(/\s+/g, ' ').trim();
	}

	function parseOpencodeEvent(message: string): Record<string, unknown> | null {
		if (!message.startsWith('{')) return null;
		try {
			const parsed = JSON.parse(message);
			if (!parsed || typeof parsed !== 'object') return null;
			if (!('type' in parsed)) return null;
			return parsed as Record<string, unknown>;
		} catch {
			return null;
		}
	}

	function levelFromExit(exitCode: number | null | undefined): AutoResolveActivityLevel {
		if (typeof exitCode !== 'number') return 'info';
		return exitCode === 0 ? 'success' : 'error';
	}

	function parseOpencodeActivity(log: SandboxJobLog, event: Record<string, unknown>): AutoResolveActivity | null {
		const type = typeof event.type === 'string' ? event.type : '';
		const part = (event.part as Record<string, unknown> | undefined) ?? {};

		switch (type) {
			case 'step_start':
				return {
					seq: log.seq,
					createdAt: log.created_at,
					title: 'Opencode step started',
					level: 'info',
					source: 'opencode'
				};
			case 'step_finish': {
				const reason =
					typeof part.reason === 'string' ? part.reason : typeof event.reason === 'string' ? event.reason : '';
				const tokenTotal =
					typeof part.tokens === 'object' && part.tokens
						? ((part.tokens as Record<string, unknown>).total as number | undefined)
						: undefined;
				const detailParts: string[] = [];
				if (reason) detailParts.push(`reason: ${reason}`);
				if (typeof tokenTotal === 'number') detailParts.push(`tokens: ${tokenTotal}`);
				return {
					seq: log.seq,
					createdAt: log.created_at,
					title: 'Opencode step finished',
					detail: detailParts.length > 0 ? detailParts.join(' | ') : undefined,
					level: reason === 'stop' ? 'success' : 'info',
					source: 'opencode'
				};
			}
			case 'tool_use': {
				const tool = typeof part.tool === 'string' ? part.tool : 'tool';
				const state = (part.state as Record<string, unknown> | undefined) ?? {};
				const status = typeof state.status === 'string' ? state.status : '';
				const metadata =
					typeof state.metadata === 'object' && state.metadata
						? (state.metadata as Record<string, unknown>)
						: {};
				const input =
					typeof state.input === 'object' && state.input ? (state.input as Record<string, unknown>) : {};
				const title =
					(typeof state.title === 'string' && state.title) ||
					(typeof metadata.description === 'string' && metadata.description) ||
					(typeof input.command === 'string' && input.command) ||
					(typeof input.filePath === 'string' && input.filePath) ||
					'';
				const exitCode = typeof metadata.exit === 'number' ? metadata.exit : undefined;
				const detailParts: string[] = [];
				if (title) detailParts.push(compactWhitespace(title));
				if (status) detailParts.push(`status: ${status}`);
				if (typeof exitCode === 'number') detailParts.push(`exit: ${exitCode}`);
				if (typeof state.error === 'string' && state.error) {
					detailParts.push(`error: ${compactWhitespace(state.error)}`);
				}
				return {
					seq: log.seq,
					createdAt: log.created_at,
					title: `Tool: ${tool}`,
					detail: detailParts.length > 0 ? truncateText(detailParts.join(' | '), 260) : undefined,
					level: status === 'error' ? 'error' : levelFromExit(exitCode),
					source: 'opencode'
				};
			}
			case 'text': {
				const text = typeof part.text === 'string' ? compactWhitespace(part.text) : '';
				if (!text) return null;
				return {
					seq: log.seq,
					createdAt: log.created_at,
					title: 'Opencode note',
					detail: truncateText(text, 300),
					level: 'info',
					source: 'opencode'
				};
			}
			case 'error': {
				const errorObj =
					typeof event.error === 'object' && event.error
						? (event.error as Record<string, unknown>)
						: {};
				const errorData =
					typeof errorObj.data === 'object' && errorObj.data
						? (errorObj.data as Record<string, unknown>)
						: {};
				const message =
					(typeof errorData.message === 'string' && errorData.message) ||
					(typeof errorObj.name === 'string' && errorObj.name) ||
					'opencode error';
				return {
					seq: log.seq,
					createdAt: log.created_at,
					title: 'Opencode error',
					detail: truncateText(compactWhitespace(message), 300),
					level: 'error',
					source: 'opencode'
				};
			}
			default:
				return null;
		}
	}

	function parseSandboxActivity(log: SandboxJobLog): AutoResolveActivity | null {
		const message = compactWhitespace(log.message);
		if (!message) return null;

		if (message.includes('still running...')) {
			return null;
		}

		if (message.startsWith('sandbox session ready:')) {
			return {
				seq: log.seq,
				createdAt: log.created_at,
				title: 'Sandbox started',
				detail: truncateText(message, 220),
				level: 'success',
				source: 'sandbox'
			};
		}

		if (message.startsWith('pull request created:')) {
			return {
				seq: log.seq,
				createdAt: log.created_at,
				title: 'Draft pull request created',
				detail: truncateText(message.replace('pull request created: ', ''), 220),
				level: 'success',
				source: 'sandbox'
			};
		}

		const commandMatch = message.match(/^command #(\d+)(?: started:|:)/);
		if (commandMatch) {
			const commandNum = commandMatch[1];
			const isStart = message.includes(' started:');
			const titleMap: Record<string, string> = {
				'1': 'Prepare opencode runtime',
				'2': 'Run opencode remediation',
				'3': 'Run validation checks'
			};
			const title = titleMap[commandNum] ?? `Run command #${commandNum}`;
			if (isStart) {
				return {
					seq: log.seq,
					createdAt: log.created_at,
					title,
					level: 'info',
					source: 'sandbox'
				};
			}
			const exit = message.match(/\(exit=(\d+)/);
			const exitCode = exit ? Number.parseInt(exit[1], 10) : undefined;
			return {
				seq: log.seq,
				createdAt: log.created_at,
				title: `${title} completed`,
				detail: typeof exitCode === 'number' ? `exit: ${exitCode}` : undefined,
				level: levelFromExit(exitCode),
				source: 'sandbox'
			};
		}

		const stageLabelPairs: Array<[string, string]> = [
			['ensure git is installed', 'Ensure git is installed'],
			['clone repository', 'Clone repository'],
			['checkout base branch', 'Checkout base branch'],
			['pull base branch', 'Pull base branch'],
			['create working branch', 'Create working branch'],
			['write incident context', 'Write incident context'],
			['write opencode prompt', 'Write opencode prompt'],
			['commit changes', 'Commit changes'],
			['push branch', 'Push branch']
		];
		for (const [needle, label] of stageLabelPairs) {
			if (!message.startsWith(needle)) continue;
			if (message.includes('(started')) {
				return {
					seq: log.seq,
					createdAt: log.created_at,
					title: label,
					level: 'info',
					source: 'sandbox'
				};
			}
			const exit = message.match(/\(exit=(\d+)/);
			const exitCode = exit ? Number.parseInt(exit[1], 10) : undefined;
			return {
				seq: log.seq,
				createdAt: log.created_at,
				title: `${label} completed`,
				detail: typeof exitCode === 'number' ? `exit: ${exitCode}` : undefined,
				level: levelFromExit(exitCode),
				source: 'sandbox'
			};
		}

		return null;
	}

	const autoResolveActivities = $derived.by((): AutoResolveActivity[] => {
		const items: AutoResolveActivity[] = [];
		for (const log of autoResolveJobLogs) {
			const opencodeEvent =
				log.stream === 'stdout' ? parseOpencodeEvent(log.message.trim()) : null;
			const activity = opencodeEvent
				? parseOpencodeActivity(log, opencodeEvent)
				: parseSandboxActivity(log);
			if (activity) {
				items.push(activity);
			}
		}
		return items.slice(-120);
	});

	function formatActivityTime(iso: string): string {
		const date = new Date(iso);
		if (Number.isNaN(date.getTime())) return '';
		return date.toLocaleTimeString('en-US', {
			hour: '2-digit',
			minute: '2-digit',
			second: '2-digit',
			hour12: false
		});
	}

	function activityTone(level: AutoResolveActivityLevel): string {
		switch (level) {
			case 'success':
				return 'border-emerald-400/40 bg-emerald-500/5';
			case 'warning':
				return 'border-amber-400/40 bg-amber-500/5';
			case 'error':
				return 'border-destructive/40 bg-destructive/10';
			default:
				return 'border-border bg-background';
		}
	}

	function sleep(ms: number): Promise<void> {
		return new Promise((resolve) => setTimeout(resolve, ms));
	}

	function resetAutoResolveJobState() {
		autoResolveJob = null;
		autoResolveJobLogs = [];
		autoResolveJobLogsCursor = 0;
		autoResolvePolling = false;
	}

	async function openAutoResolve(log: LogRecord) {
		if (!orgId || autoResolveBusyLogID) return;

		autoResolveBusyLogID = log.id;
		autoResolveError = undefined;
		autoResolveMissingService = undefined;
		autoResolvePreview = null;
		resetAutoResolveJobState();

		try {
			autoResolvePreview = await getLogAutoResolvePreview(orgId, log.id);
			autoResolveDialogOpen = true;
		} catch (error) {
			if (error instanceof RemediationAPIError) {
				if (error.code === 'auto_resolve_service_mapping_missing') {
					const detail = error.details.find((d) => d.location === 'auto_resolve.service_name');
					autoResolveMissingService = typeof detail?.value === 'string' ? detail.value : log.service_name;
					autoResolveDialogOpen = true;
				} else {
					autoResolveError = error.code;
				}
			} else {
				autoResolveError = error instanceof Error ? error.message : 'failed to preview auto resolve';
			}
		} finally {
			autoResolveBusyLogID = undefined;
		}
	}

	async function pollAutoResolveJob(jobID: string) {
		if (!orgId || !jobID || autoResolvePolling) return;

		autoResolvePolling = true;
		autoResolveError = undefined;
		try {
			while (true) {
				const jobResp = await getSandboxJob(orgId, jobID);
				if (jobResp.status !== 200) {
					autoResolveError =
						(jobResp.data as { detail?: string })?.detail ?? 'failed to fetch auto resolve job';
					break;
				}
				autoResolveJob = jobResp.data;

				const logsResp = await getSandboxJobLogs(orgId, jobID, {
					cursor: autoResolveJobLogsCursor,
					limit: 200
				});
				if (logsResp.status === 200) {
					const nextLogs = logsResp.data.logs ?? [];
					if (nextLogs.length > 0) {
						autoResolveJobLogs = [...autoResolveJobLogs, ...nextLogs];
					}
					autoResolveJobLogsCursor = logsResp.data.next_cursor ?? autoResolveJobLogsCursor;
				}

				if (
					autoResolveJob.state === 'succeeded' ||
					autoResolveJob.state === 'failed' ||
					autoResolveJob.state === 'cancelled' ||
					autoResolveJob.state === 'timed_out'
				) {
					break;
				}

				await sleep(2000);
			}
		} catch (error) {
			autoResolveError = error instanceof Error ? error.message : 'failed to poll auto resolve job';
		} finally {
			autoResolvePolling = false;
			autoResolveBusyLogID = undefined;
		}
	}

	async function startAutoResolve() {
		if (!orgId || !autoResolvePreview || autoResolveRunLoading || autoResolvePolling) return;

		autoResolveRunLoading = true;
		autoResolveError = undefined;
		autoResolveBusyLogID = autoResolvePreview.log_id;
		resetAutoResolveJobState();
		try {
			const out = await runLogAutoResolve(orgId, autoResolvePreview.log_id);
			await pollAutoResolveJob(out.job_id);
		} catch (error) {
			if (error instanceof RemediationAPIError) {
				autoResolveError = error.code;
			} else {
				autoResolveError = error instanceof Error ? error.message : 'failed to start auto resolve';
			}
		} finally {
			autoResolveRunLoading = false;
		}
	}
</script>

<PageContainer breadcrumbs={[{ title: 'Logs', href: '/logs' }, { title: 'All Logs' }]}>
	<div class="flex h-full flex-col gap-4">
		<!-- Header -->
		<div class="flex items-start justify-between">
			<div class="flex items-center gap-3">
				<div class="rounded-lg bg-primary/10 p-2">
					<ListIcon class="size-6 text-primary" />
				</div>
				<div>
					<h1 class="text-xl font-semibold">Logging</h1>
					<p class="text-sm text-muted-foreground">Search, filter and debug your logs</p>
				</div>
			</div>
			<Button onclick={saveView}>
				<BookmarkIcon class="mr-2 size-4" />
				Save view
			</Button>
		</div>

		<Separator />

		<!-- Filters -->
		<LogFilters
			bind:searchQuery
			bind:selectedService
			bind:selectedLevel
			bind:selectedTimeRange
			{services}
			onSearch={handleSearch}
		/>

		{#if restoreRequired}
			<Alert.Root class="border-amber-400/40 bg-amber-50/40 dark:bg-amber-950/20">
				<Alert.Title>Archived data required</Alert.Title>
				<Alert.Description>
					Requested range includes archived days:
					{restoreRequired.missingDays.join(', ') ||
						`${restoreRequired.startDay} to ${restoreRequired.endDay}`}. Restore and merge missing
					data into `*_restored` tables.
				</Alert.Description>
				<div class="mt-3 flex flex-wrap items-center gap-2">
					<Button size="sm" onclick={startRestore} disabled={isStartingRestore || isPollingRestore}>
						{#if isStartingRestore}
							Starting restore...
						{:else if isPollingRestore}
							Restoring...
						{:else}
							Restore Archived Days
						{/if}
					</Button>
					{#if restoreJob}
						<span class="text-xs text-muted-foreground">
							{restoreJob.completed_items}/{restoreJob.total_items} objects restored
						</span>
					{/if}
					{#if restoreError}
						<span class="text-xs text-destructive">{restoreError}</span>
					{/if}
				</div>
			</Alert.Root>
		{/if}

		{#if autoResolveError && !autoResolveDialogOpen}
			<Alert.Root class="border-destructive/40 bg-destructive/10">
				<Alert.Title>Auto Resolve failed</Alert.Title>
				<Alert.Description>{autoResolveError}</Alert.Description>
			</Alert.Root>
		{/if}

		<!-- Histogram Chart -->
		<LogHistogram
			data={histogramData}
			visible={chartVisible}
			loading={histogramQuery.isPending}
			onToggleVisibility={() => (chartVisible = !chartVisible)}
		/>

		<!-- Log List -->
		<div class="min-h-0 flex-1">
			<LogList
				logs={allLogs}
				{isLive}
				onToggleLive={() => (isLive = !isLive)}
				onLoadMore={hasMore ? loadMore : undefined}
				{hasMore}
				loading={logsQuery.isPending || isLoadingMore}
				onAutoResolve={openAutoResolve}
				{autoResolveBusyLogID}
			/>
		</div>
	</div>

	<Dialog.Root bind:open={autoResolveDialogOpen}>
		<Dialog.Content class="sm:max-w-3xl">
			<Dialog.Header>
				<Dialog.Title>Auto Resolve</Dialog.Title>
				<Dialog.Description>
					Run an automated remediation in the Docker sandbox and open a draft pull request.
				</Dialog.Description>
			</Dialog.Header>

			{#if autoResolveMissingService}
				<Alert.Root class="border-amber-400/40 bg-amber-50/50 dark:bg-amber-950/20">
					<Alert.Title>Service mapping required</Alert.Title>
					<Alert.Description>
						No repository mapping is configured for service
						<code class="rounded bg-muted px-1 py-0.5 text-xs">{autoResolveMissingService}</code>.
						Configure mappings in Settings before running Auto Resolve.
					</Alert.Description>
				</Alert.Root>
				<div class="mt-4 flex justify-end">
					<a class="text-sm text-primary underline" href="/settings#remediation-mappings">
						Open Service Mappings
					</a>
				</div>
			{:else if autoResolvePreview}
				<div class="space-y-4">
					<div class="grid gap-3 text-sm md:grid-cols-2">
						<div><span class="text-muted-foreground">Service:</span> {autoResolvePreview.service_name}</div>
						<div><span class="text-muted-foreground">Log ID:</span> {autoResolvePreview.log_id}</div>
						<div>
							<span class="text-muted-foreground">Repository:</span>
							{autoResolvePreview.repository_full_name}
						</div>
						<div>
							<span class="text-muted-foreground">Base Branch:</span>
							{autoResolvePreview.base_branch}
						</div>
						<div class="md:col-span-2">
							<span class="text-muted-foreground">Task Title:</span>
							{autoResolvePreview.task_title}
						</div>
						<div class="md:col-span-2">
							<span class="text-muted-foreground">Commit Message:</span>
							{autoResolvePreview.commit_message}
						</div>
						<div>
							<span class="text-muted-foreground">Trace Spans:</span>
							{autoResolvePreview.context_summary.trace_span_count}
						</div>
						<div>
							<span class="text-muted-foreground">Nearby Errors:</span>
							{autoResolvePreview.context_summary.nearby_error_log_count}
						</div>
					</div>

					<div>
						<div class="mb-2 text-sm font-medium">Validation Commands</div>
						<div class="rounded-md border bg-muted/40 p-2 font-mono text-xs">
							{#each autoResolvePreview.validation_commands as command (command)}
								<div class="break-all whitespace-pre-wrap">{command}</div>
							{/each}
						</div>
					</div>

					{#if autoResolveJob}
						<div class="rounded-md border p-3">
							<div class="mb-2 flex items-center justify-between">
								<div class="text-sm font-medium">Job {autoResolveJob.id}</div>
								<Badge variant="secondary">{formatSandboxState(autoResolveJob.state)}</Badge>
							</div>
								<div class="space-y-1 text-xs text-muted-foreground">
									<div>Branch: {autoResolveJob.branch_name || 'pending'}</div>
									{#if autoResolveJob.pull_request_url}
									<div>
										Pull Request:
										<a
											class="text-primary underline"
											href={autoResolveJob.pull_request_url}
											target="_blank"
											rel="noreferrer"
										>
											{autoResolveJob.pull_request_url}
										</a>
										</div>
									{/if}
								</div>
								<div class="mt-3 space-y-3">
									<div class="space-y-2">
										<div class="flex items-center justify-between">
											<div class="text-xs font-medium">Opencode Activity</div>
											<Badge variant="outline" class="text-[10px]">
												{autoResolveActivities.length} events
											</Badge>
										</div>
										<div class="max-h-56 space-y-2 overflow-auto rounded-md border bg-muted/20 p-2">
											{#if autoResolveActivities.length === 0}
												<div class="text-xs text-muted-foreground">
													{#if autoResolveJob.state === 'queued' || autoResolveJob.state === 'running'}
														Job is active. Waiting for activity...
													{:else}
														No activity captured.
													{/if}
												</div>
											{:else}
												{#each autoResolveActivities as activity (activity.seq)}
													<div class={`rounded-md border p-2 text-xs ${activityTone(activity.level)}`}>
														<div class="flex items-center justify-between gap-2">
															<div class="font-medium">{activity.title}</div>
															<div class="text-[10px] text-muted-foreground">
																{activity.source} | {formatActivityTime(activity.createdAt)}
															</div>
														</div>
														{#if activity.detail}
															<div class="mt-1 break-all whitespace-pre-wrap font-mono text-[11px] text-muted-foreground">
																{activity.detail}
															</div>
														{/if}
													</div>
												{/each}
											{/if}
										</div>
									</div>

									<details class="rounded-md border bg-muted/10 p-2">
										<summary class="cursor-pointer text-xs font-medium text-muted-foreground">
											Raw sandbox logs ({autoResolveJobLogs.length})
										</summary>
										<div class="mt-2 max-h-40 overflow-auto rounded-md bg-muted p-2 font-mono text-[11px]">
											{#if autoResolveJobLogs.length === 0}
												<div class="text-muted-foreground">No raw logs yet.</div>
											{:else}
												{#each autoResolveJobLogs as log (log.seq)}
													<div class="break-all whitespace-pre-wrap">[{log.stream}] {truncateText(log.message, 800)}</div>
												{/each}
											{/if}
										</div>
									</details>
								</div>
							</div>
						{/if}
					</div>

				{#if autoResolveError}
					<div class="mt-3 rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
						{autoResolveError}
					</div>
				{/if}

				<Dialog.Footer class="mt-4">
					<Button variant="outline" onclick={() => (autoResolveDialogOpen = false)}>
						Close
					</Button>
					<Button onclick={startAutoResolve} disabled={autoResolveRunLoading || autoResolvePolling}>
						{#if autoResolveRunLoading}
							Starting...
						{:else if autoResolvePolling}
							Running...
						{:else}
							Run Auto Resolve
						{/if}
					</Button>
				</Dialog.Footer>
			{:else}
				<div class="text-sm text-muted-foreground">Preparing auto resolve preview...</div>
			{/if}
		</Dialog.Content>
	</Dialog.Root>
</PageContainer>
