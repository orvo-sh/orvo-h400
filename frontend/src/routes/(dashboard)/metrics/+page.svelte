<script lang="ts">
	import PageContainer from '../_components/page-container/page-container.svelte';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Separator } from '$lib/components/ui/separator/index.js';
	import * as Select from '$lib/components/ui/select/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import SearchIcon from '@lucide/svelte/icons/search';
	import ChartLineIcon from '@lucide/svelte/icons/chart-line';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import MetricChart from './_components/metric-chart.svelte';
	import MetricCatalogList from './_components/metric-catalog-list.svelte';
	import {
		createGetMetricCatalog,
		createQueryTimeseries,
		recalculateDerivedMetrics
	} from '$lib/api/endpoints/metrics/metrics';
	import { sessionStore } from '$lib/stores/session';
	import type { MetricMeta, Timeseries } from '$lib/api/model';

	let searchQuery = $state('');
	let selectedMetric = $state<string | undefined>(undefined);
	let selectedService = $state<string | undefined>(undefined);
	let selectedTimeRange = $state('1h');
	let selectedAggregation = $state('avg');
	let selectedStep = $state('');
	let groupBy = $state('');
	let recalculating = $state(false);
	let recalcStatus = $state('');

	const orgId = $derived($sessionStore?.active_organization?.id ?? '');

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

	const defaultSteps: Record<string, string> = {
		'15m': '1m',
		'30m': '1m',
		'1h': '1m',
		'3h': '5m',
		'6h': '5m',
		'12h': '15m',
		'24h': '30m',
		'7d': '1h'
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
	}

	function rangeToLookback(range: string): string {
		switch (range) {
			case '15m':
				return '15m';
			case '30m':
				return '30m';
			case '1h':
				return '1h';
			case '3h':
				return '3h';
			case '6h':
				return '6h';
			case '12h':
				return '12h';
			case '24h':
				return '24h';
			case '7d':
				return '168h';
			default:
				return '1h';
		}
	}

	$effect(() => {
		selectedTimeRange;
		refreshQueries();
	});

	const catalogQuery = createGetMetricCatalog(
		() => orgId,
		() => ({
			service: selectedService || undefined,
			search: searchQuery || undefined
		})
	);

	const metrics = $derived.by((): MetricMeta[] => {
		const resp = catalogQuery.data;
		if (resp && resp.status === 200) {
			return resp.data.metrics ?? [];
		}
		return [];
	});

	const effectiveStep = $derived(selectedStep || defaultSteps[selectedTimeRange] || '1m');

	const timeseriesQuery = createQueryTimeseries(
		() => orgId,
		() => ({
			metric: selectedMetric ?? '',
			start: queryStart,
			end: queryEnd,
			step: effectiveStep,
			aggregation: selectedAggregation || undefined,
			service: selectedService || undefined,
			group_by: groupBy || undefined
		}),
		() => ({
			query: {
				enabled: !!selectedMetric && !!orgId
			}
		})
	);

	const series = $derived.by((): Timeseries[] => {
		const resp = timeseriesQuery.data;
		if (resp && resp.status === 200) {
			return resp.data.series ?? [];
		}
		return [];
	});

	function selectMetric(name: string) {
		selectedMetric = name;
		refreshQueries();
	}

	async function refreshAll() {
		refreshQueries();
		await catalogQuery.refetch();
		if (selectedMetric) {
			await timeseriesQuery.refetch();
		}
	}

	async function recalculateDerivedNow() {
		if (!orgId || recalculating) {
			return;
		}

		recalculating = true;
		recalcStatus = '';
		try {
			const response = await recalculateDerivedMetrics(orgId, {
				service: selectedService || undefined,
				lookback: rangeToLookback(selectedTimeRange)
			});
			await refreshAll();

			if (response.status === 200) {
				const asOf = response.data.as_of ? new Date(response.data.as_of).toLocaleTimeString() : 'now';
				recalcStatus = `Derived metrics recalculated (as of ${asOf})`;
			} else {
				recalcStatus = 'Derived metrics recalculation failed';
			}
		} catch {
			recalcStatus = 'Derived metrics recalculation failed';
		} finally {
			recalculating = false;
		}
	}
</script>

<PageContainer breadcrumbs={[{ title: 'Metrics', href: '/metrics' }, { title: 'Explorer' }]}>
	<div class="flex h-full flex-col gap-4">
		<!-- Header -->
		<div class="flex items-start justify-between">
			<div class="flex items-center gap-3">
				<div class="rounded-lg bg-primary/10 p-2">
					<ChartLineIcon class="size-6 text-primary" />
				</div>
				<div>
					<h1 class="text-xl font-semibold">Metrics Explorer</h1>
					<p class="text-sm text-muted-foreground">Browse, query, and visualize your metrics</p>
				</div>
			</div>
			<div class="flex items-center gap-2">
				<Button variant="outline" onclick={recalculateDerivedNow} disabled={recalculating || !orgId}>
					<RefreshCwIcon class="mr-2 size-4 {recalculating ? 'animate-spin' : ''}" />
					Recalculate Derived
				</Button>
				<Button onclick={refreshAll}>
					<RefreshCwIcon class="mr-2 size-4" />
					Refresh
				</Button>
			</div>
		</div>

		{#if recalcStatus}
			<p class="text-xs text-muted-foreground">{recalcStatus}</p>
		{/if}

		<Separator />

		<div class="flex min-h-0 flex-1 gap-4">
			<!-- Metric catalog sidebar -->
			<div class="flex w-72 shrink-0 flex-col gap-3 border-r pr-4">
				<div class="relative">
					<SearchIcon class="absolute left-2.5 top-2.5 size-4 text-muted-foreground" />
					<Input
						placeholder="Search metrics..."
						class="pl-9"
						bind:value={searchQuery}
					/>
				</div>
				<MetricCatalogList
					{metrics}
					loading={catalogQuery.isPending}
					selected={selectedMetric}
					onSelect={selectMetric}
				/>
			</div>

			<!-- Chart area -->
			<div class="flex min-w-0 flex-1 flex-col gap-3">
				<!-- Query controls -->
				<div class="flex flex-wrap items-center gap-2">
					<Select.Root
						type="single"
						value={selectedTimeRange}
						onValueChange={(v) => { if (v) selectedTimeRange = v; }}
					>
						<Select.Trigger class="w-28">
							{selectedTimeRange || 'Time range'}
						</Select.Trigger>
						<Select.Content>
							{#each ['15m', '30m', '1h', '3h', '6h', '12h', '24h', '7d'] as range}
								<Select.Item value={range}>{range}</Select.Item>
							{/each}
						</Select.Content>
					</Select.Root>

					<Select.Root
						type="single"
						value={selectedAggregation}
						onValueChange={(v) => { if (v) selectedAggregation = v; }}
					>
						<Select.Trigger class="w-28">
							{selectedAggregation || 'Aggregation'}
						</Select.Trigger>
						<Select.Content>
							{#each ['avg', 'sum', 'min', 'max', 'rate', 'count', 'last', 'p50', 'p90', 'p95', 'p99'] as agg}
								<Select.Item value={agg}>{agg}</Select.Item>
							{/each}
						</Select.Content>
					</Select.Root>

					<Select.Root
						type="single"
						value={selectedStep || ''}
						onValueChange={(v) => { selectedStep = v ?? ''; }}
					>
						<Select.Trigger class="w-28">
							{selectedStep || `auto (${defaultSteps[selectedTimeRange] || '1m'})`}
						</Select.Trigger>
						<Select.Content>
							<Select.Item value="">Auto</Select.Item>
							{#each ['1m', '5m', '15m', '30m', '1h', '6h', '1d'] as step}
								<Select.Item value={step}>{step}</Select.Item>
							{/each}
						</Select.Content>
					</Select.Root>

					<Input
						placeholder="Group by (e.g. service.name)"
						class="w-52"
						bind:value={groupBy}
					/>
				</div>

				<!-- Chart -->
				{#if selectedMetric}
					<MetricChart
						{series}
						metricName={selectedMetric}
						loading={timeseriesQuery.isPending}
					/>
				{:else}
					<div class="flex flex-1 items-center justify-center rounded-lg border border-dashed">
						<p class="text-sm text-muted-foreground">Select a metric from the catalog to start exploring</p>
					</div>
				{/if}
			</div>
		</div>
	</div>
</PageContainer>
