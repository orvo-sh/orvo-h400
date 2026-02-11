<script lang="ts">
	import { goto } from '$app/navigation';
	import PageContainer from '../_components/page-container/page-container.svelte';
	import { Separator } from '$lib/components/ui/separator/index.js';
	import ActivityIcon from '@lucide/svelte/icons/activity';
	import TraceFilters from './_components/trace-filters.svelte';
	import TraceList from './_components/trace-list.svelte';
	import {
		createQueryTraces,
		createGetTraceServices,
		queryTraces
	} from '$lib/api/endpoints/traces/traces';
	import { sessionStore } from '$lib/stores/session';
	import type { TraceSummary } from '$lib/api/model';

	type StatusFilter = 'ok' | 'error' | 'unset';

	// Filter UI state
	let searchQuery = $state('');
	let selectedService = $state<string | undefined>(undefined);
	let selectedStatus = $state<StatusFilter | undefined>(undefined);
	let selectedTimeRange = $state('1h');
	let minDuration = $state<number | undefined>(undefined);
	let maxDuration = $state<number | undefined>(undefined);

	// Pagination state
	let extraTraces = $state<TraceSummary[]>([]);
	let nextCursor = $state<string | undefined>(undefined);
	let isLoadingMore = $state(false);

	// Committed search -- only updates on Enter
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
		extraTraces = [];
		nextCursor = undefined;
	}

	// Re-run when dropdown filters change
	$effect(() => {
		selectedTimeRange;
		selectedService;
		selectedStatus;
		minDuration;
		maxDuration;
		refreshQueries();
	});

	function handleSearch() {
		committedSearch = searchQuery;
		refreshQueries();
	}

	// API queries
	const tracesQuery = createQueryTraces(
		() => orgId,
		() => ({
			start: queryStart,
			end: queryEnd,
			limit: 50,
			search: committedSearch || undefined,
			service: selectedService || undefined,
			status: selectedStatus || undefined,
			min_duration: minDuration,
			max_duration: maxDuration
		})
	);

	const servicesQuery = createGetTraceServices(() => orgId);

	// Extract data from query results
	const firstPageTraces = $derived.by((): TraceSummary[] => {
		const resp = tracesQuery.data;
		if (resp && resp.status === 200) {
			return resp.data.traces ?? [];
		}
		return [];
	});

	// Track next_cursor from initial query
	$effect(() => {
		const resp = tracesQuery.data;
		if (resp && resp.status === 200 && extraTraces.length === 0) {
			nextCursor = resp.data.next_cursor;
		}
	});

	const allTraces = $derived([...firstPageTraces, ...extraTraces]);
	const hasMore = $derived(!!nextCursor);

	const services = $derived.by((): string[] => {
		const resp = servicesQuery.data;
		if (resp && resp.status === 200) {
			return resp.data.services ?? [];
		}
		return [];
	});

	// Load more traces (cursor-based pagination)
	async function loadMore() {
		if (!nextCursor || isLoadingMore) return;
		isLoadingMore = true;
		try {
			const resp = await queryTraces(orgId, {
				start: queryStart,
				end: queryEnd,
				limit: 50,
				cursor: nextCursor,
				search: committedSearch || undefined,
				service: selectedService || undefined,
				status: selectedStatus || undefined,
				min_duration: minDuration,
				max_duration: maxDuration
			});
			if (resp.status === 200) {
				extraTraces = [...extraTraces, ...(resp.data.traces ?? [])];
				nextCursor = resp.data.next_cursor;
			}
		} finally {
			isLoadingMore = false;
		}
	}

	function handleTraceClick(traceId: string) {
		goto(`/traces/${traceId}`);
	}
</script>

<PageContainer breadcrumbs={[{ title: 'Traces', href: '/traces' }, { title: 'All Traces' }]}>
	<div class="flex h-full flex-col gap-4">
		<!-- Header -->
		<div class="flex items-start justify-between">
			<div class="flex items-center gap-3">
				<div class="rounded-lg bg-primary/10 p-2">
					<ActivityIcon class="size-6 text-primary" />
				</div>
				<div>
					<h1 class="text-xl font-semibold">Traces</h1>
					<p class="text-sm text-muted-foreground">
						Explore distributed traces across your services
					</p>
				</div>
			</div>
		</div>

		<Separator />

		<!-- Filters -->
		<TraceFilters
			bind:searchQuery
			bind:selectedService
			bind:selectedStatus
			bind:selectedTimeRange
			bind:minDuration
			bind:maxDuration
			{services}
			onSearch={handleSearch}
		/>

		<!-- Trace List -->
		<div class="min-h-0 flex-1">
			<TraceList
				traces={allTraces}
				onTraceClick={handleTraceClick}
				onLoadMore={hasMore ? loadMore : undefined}
				{hasMore}
				loading={tracesQuery.isPending || isLoadingMore}
			/>
		</div>
	</div>
</PageContainer>
