<script lang="ts">
	import PageContainer from '../../_components/page-container/page-container.svelte';
	import { Separator } from '$lib/components/ui/separator/index.js';
	import * as Select from '$lib/components/ui/select/index.js';
	import * as Card from '$lib/components/ui/card/index.js';
	import ActivityIcon from '@lucide/svelte/icons/activity';
	import RedChart from './_components/red-chart.svelte';
	import {
		createGetRedMetrics
	} from '$lib/api/endpoints/metrics/metrics';
	import {
		createGetTraceServices
	} from '$lib/api/endpoints/traces/traces';
	import { sessionStore } from '$lib/stores/session';
	import type { TimeseriesPoint } from '$lib/api/model';

	let selectedService = $state('');
	let selectedTimeRange = $state('1h');

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

	$effect(() => {
		selectedTimeRange;
		refreshQueries();
	});

	const servicesQuery = createGetTraceServices(() => orgId);

	const services = $derived.by((): string[] => {
		const resp = servicesQuery.data;
		if (resp && resp.status === 200) {
			return resp.data.services ?? [];
		}
		return [];
	});

	// Auto-select first service
	$effect(() => {
		if (!selectedService && services.length > 0) {
			selectedService = services[0];
		}
	});

	const redQuery = createGetRedMetrics(
		() => orgId,
		() => ({
			service: selectedService,
			start: queryStart,
			end: queryEnd,
			step: defaultSteps[selectedTimeRange] || '1m'
		}),
		() => ({
			query: {
				enabled: !!selectedService && !!orgId
			}
		})
	);

	const requestRate = $derived.by((): TimeseriesPoint[] => {
		const resp = redQuery.data;
		if (resp && resp.status === 200) return resp.data.request_rate ?? [];
		return [];
	});

	const errorRate = $derived.by((): TimeseriesPoint[] => {
		const resp = redQuery.data;
		if (resp && resp.status === 200) return resp.data.error_rate ?? [];
		return [];
	});

	const p50 = $derived.by((): TimeseriesPoint[] => {
		const resp = redQuery.data;
		if (resp && resp.status === 200) return resp.data.p50_latency ?? [];
		return [];
	});

	const p90 = $derived.by((): TimeseriesPoint[] => {
		const resp = redQuery.data;
		if (resp && resp.status === 200) return resp.data.p90_latency ?? [];
		return [];
	});

	const p95 = $derived.by((): TimeseriesPoint[] => {
		const resp = redQuery.data;
		if (resp && resp.status === 200) return resp.data.p95_latency ?? [];
		return [];
	});

	const p99 = $derived.by((): TimeseriesPoint[] => {
		const resp = redQuery.data;
		if (resp && resp.status === 200) return resp.data.p99_latency ?? [];
		return [];
	});

	// Summary stats
	function lastValue(points: TimeseriesPoint[]): string {
		if (points.length === 0) return '-';
		const v = points[points.length - 1].value;
		if (Math.abs(v) >= 1000) return (v / 1000).toFixed(1) + 'K';
		return v.toFixed(2);
	}
</script>

<PageContainer breadcrumbs={[{ title: 'Metrics', href: '/metrics' }, { title: 'Services' }]}>
	<div class="flex h-full flex-col gap-4">
		<!-- Header -->
		<div class="flex items-start justify-between">
			<div class="flex items-center gap-3">
				<div class="rounded-lg bg-primary/10 p-2">
					<ActivityIcon class="size-6 text-primary" />
				</div>
				<div>
					<h1 class="text-xl font-semibold">Service Overview</h1>
					<p class="text-sm text-muted-foreground">Rate, Errors, and Duration (RED) metrics for your services</p>
				</div>
			</div>
		</div>

		<Separator />

		<!-- Controls -->
		<div class="flex items-center gap-3">
			<Select.Root
				type="single"
				value={selectedService}
				onValueChange={(v) => { if (v) selectedService = v; }}
			>
				<Select.Trigger class="w-64">
					{selectedService || 'Select a service'}
				</Select.Trigger>
				<Select.Content>
					{#each services as svc}
						<Select.Item value={svc}>{svc}</Select.Item>
					{/each}
				</Select.Content>
			</Select.Root>

			<Select.Root
				type="single"
				value={selectedTimeRange}
				onValueChange={(v) => { if (v) selectedTimeRange = v; }}
			>
				<Select.Trigger class="w-28">
					{selectedTimeRange}
				</Select.Trigger>
				<Select.Content>
					{#each ['15m', '30m', '1h', '3h', '6h', '12h', '24h', '7d'] as range}
						<Select.Item value={range}>{range}</Select.Item>
					{/each}
				</Select.Content>
			</Select.Root>
		</div>

		{#if !selectedService}
			<div class="flex flex-1 items-center justify-center rounded-lg border border-dashed">
				<p class="text-sm text-muted-foreground">Select a service to view RED metrics</p>
			</div>
		{:else}
			<!-- Summary cards -->
			<div class="grid grid-cols-4 gap-4">
				<Card.Root>
					<Card.Header class="pb-2">
						<Card.Title class="text-sm font-medium text-muted-foreground">Request Rate</Card.Title>
					</Card.Header>
					<Card.Content>
						<p class="text-2xl font-bold">{lastValue(requestRate)}<span class="text-sm font-normal text-muted-foreground"> req/s</span></p>
					</Card.Content>
				</Card.Root>
				<Card.Root>
					<Card.Header class="pb-2">
						<Card.Title class="text-sm font-medium text-muted-foreground">Error Rate</Card.Title>
					</Card.Header>
					<Card.Content>
						<p class="text-2xl font-bold">{lastValue(errorRate)}<span class="text-sm font-normal text-muted-foreground"> err/s</span></p>
					</Card.Content>
				</Card.Root>
				<Card.Root>
					<Card.Header class="pb-2">
						<Card.Title class="text-sm font-medium text-muted-foreground">P50 Latency</Card.Title>
					</Card.Header>
					<Card.Content>
						<p class="text-2xl font-bold">{lastValue(p50)}<span class="text-sm font-normal text-muted-foreground"> ms</span></p>
					</Card.Content>
				</Card.Root>
				<Card.Root>
					<Card.Header class="pb-2">
						<Card.Title class="text-sm font-medium text-muted-foreground">P99 Latency</Card.Title>
					</Card.Header>
					<Card.Content>
						<p class="text-2xl font-bold">{lastValue(p99)}<span class="text-sm font-normal text-muted-foreground"> ms</span></p>
					</Card.Content>
				</Card.Root>
			</div>

			<!-- Charts -->
			<div class="grid grid-cols-2 gap-4">
				<RedChart title="Request Rate (req/s)" points={requestRate} color="var(--color-primary)" loading={redQuery.isPending} />
				<RedChart title="Error Rate (err/s)" points={errorRate} color="#ef4444" loading={redQuery.isPending} />
				<RedChart title="Latency (P50 / P90)" points={p50} secondaryPoints={p90} color="#10b981" secondaryColor="#f59e0b" loading={redQuery.isPending} />
				<RedChart title="Latency (P95 / P99)" points={p95} secondaryPoints={p99} color="#8b5cf6" secondaryColor="#ef4444" loading={redQuery.isPending} />
			</div>
		{/if}
	</div>
</PageContainer>
