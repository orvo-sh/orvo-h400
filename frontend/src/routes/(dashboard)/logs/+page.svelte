<script lang="ts">
	import PageContainer from '../_components/page-container/page-container.svelte';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Separator } from '$lib/components/ui/separator/index.js';
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
	import { sessionStore } from '$lib/stores/session';
	import type { LogRecord, HistogramBucket } from '$lib/api/model';

	type LogLevel = 'debug' | 'info' | 'warn' | 'error' | 'fatal';

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
		'7d': 7 * 24 * 60 * 60 * 1000
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

	function saveView() {
		console.log('Save view clicked');
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
			/>
		</div>
	</div>
</PageContainer>
