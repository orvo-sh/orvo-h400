<script lang="ts">
	import PageContainer from '../../_components/page-container/page-container.svelte';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import * as Table from '$lib/components/ui/table/index.js';
	import { Separator } from '$lib/components/ui/separator/index.js';
	import DatabaseIcon from '@lucide/svelte/icons/database';
	import ActivityIcon from '@lucide/svelte/icons/activity';
	import AlertTriangleIcon from '@lucide/svelte/icons/alert-triangle';
	import ClockIcon from '@lucide/svelte/icons/clock';
	import { createGetTraceSources } from '$lib/api/endpoints/traces/traces';
	import { sessionStore } from '$lib/stores/session';
	import type { ServiceSource } from '$lib/api/model';

	const orgId = $derived($sessionStore?.active_organization?.id ?? '');
	const sourcesQuery = createGetTraceSources(() => orgId);

	const sources = $derived.by((): ServiceSource[] => {
		const resp = sourcesQuery.data;
		if (resp && resp.status === 200) {
			return resp.data.sources ?? [];
		}
		return [];
	});

	// Summary stats
	const totalSources = $derived(sources.length);
	const totalSpans = $derived(sources.reduce((sum, s) => sum + s.span_count, 0));
	const totalErrors = $derived(sources.reduce((sum, s) => sum + s.error_count, 0));
	const overallErrorRate = $derived(totalSpans > 0 ? (totalErrors / totalSpans) * 100 : 0);

	function formatDuration(ns: number): string {
		const ms = ns / 1_000_000;
		if (ms < 1) return `${(ns / 1_000).toFixed(0)}us`;
		if (ms < 1000) return `${ms.toFixed(1)}ms`;
		return `${(ms / 1000).toFixed(2)}s`;
	}

	function formatCount(n: number): string {
		if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
		if (n >= 1_000) return `${(n / 1_000).toFixed(1)}k`;
		return n.toString();
	}

	function formatRelativeTime(iso: string): string {
		const now = Date.now();
		const then = new Date(iso).getTime();
		const diffMs = now - then;

		if (diffMs < 0) return 'just now';
		const seconds = Math.floor(diffMs / 1000);
		if (seconds < 60) return `${seconds}s ago`;
		const minutes = Math.floor(seconds / 60);
		if (minutes < 60) return `${minutes}m ago`;
		const hours = Math.floor(minutes / 60);
		if (hours < 24) return `${hours}h ago`;
		const days = Math.floor(hours / 24);
		return `${days}d ago`;
	}

	function getErrorRateColor(errorRate: number): string {
		if (errorRate === 0) return 'text-emerald-600';
		if (errorRate < 1) return 'text-amber-600';
		if (errorRate < 5) return 'text-orange-600';
		return 'text-red-600';
	}

	function getErrorBadgeVariant(errorRate: number): 'default' | 'secondary' | 'destructive' {
		if (errorRate === 0) return 'default';
		if (errorRate < 5) return 'secondary';
		return 'destructive';
	}
</script>

<PageContainer breadcrumbs={[{ title: 'Traces', href: '/traces' }, { title: 'Sources' }]}>
	<div class="flex flex-col gap-6">
		<!-- Header -->
		<div class="flex items-start justify-between">
			<div class="flex items-center gap-3">
				<div class="rounded-lg bg-primary/10 p-2">
					<DatabaseIcon class="size-6 text-primary" />
				</div>
				<div>
					<h1 class="text-xl font-semibold">Trace Sources</h1>
					<p class="text-sm text-muted-foreground">
						Services sending trace data in the last 24 hours
					</p>
				</div>
			</div>
		</div>

		{#if sourcesQuery.isPending}
			<div class="flex h-64 items-center justify-center text-muted-foreground">
				Loading sources...
			</div>
		{:else if sources.length === 0}
			<div class="flex h-64 items-center justify-center rounded-lg border border-dashed">
				<div class="text-center">
					<DatabaseIcon class="mx-auto mb-3 size-12 text-muted-foreground/50" />
					<h3 class="text-lg font-medium text-muted-foreground">No trace sources</h3>
					<p class="mt-1 text-sm text-muted-foreground/70">
						No services have sent trace data in the last 24 hours
					</p>
				</div>
			</div>
		{:else}
			<!-- Summary Stats -->
			<div class="grid gap-4 md:grid-cols-4">
				<div class="rounded-lg border p-4">
					<div class="flex items-center gap-2 text-sm text-muted-foreground">
						<DatabaseIcon class="size-4" />
						<span>Total Services</span>
					</div>
					<p class="mt-1 text-2xl font-semibold">{totalSources}</p>
				</div>
				<div class="rounded-lg border p-4">
					<div class="flex items-center gap-2 text-sm text-muted-foreground">
						<ActivityIcon class="size-4" />
						<span>Total Spans (24h)</span>
					</div>
					<p class="mt-1 font-mono text-2xl font-semibold">{formatCount(totalSpans)}</p>
				</div>
				<div class="rounded-lg border p-4">
					<div class="flex items-center gap-2 text-sm text-muted-foreground">
						<AlertTriangleIcon class="size-4" />
						<span>Total Errors (24h)</span>
					</div>
					<p class="mt-1 font-mono text-2xl font-semibold text-red-600">
						{formatCount(totalErrors)}
					</p>
				</div>
				<div class="rounded-lg border p-4">
					<div class="flex items-center gap-2 text-sm text-muted-foreground">
						<ClockIcon class="size-4" />
						<span>Error Rate</span>
					</div>
					<p class="mt-1 font-mono text-2xl font-semibold {getErrorRateColor(overallErrorRate)}">
						{overallErrorRate.toFixed(2)}%
					</p>
				</div>
			</div>

			<!-- Sources Table -->
			<div class="rounded-lg border">
				<Table.Root>
					<Table.Header>
						<Table.Row>
							<Table.Head class="w-[250px]">Service Name</Table.Head>
							<Table.Head class="text-right">Spans (24h)</Table.Head>
							<Table.Head class="text-right">Errors</Table.Head>
							<Table.Head class="text-right">Error Rate</Table.Head>
							<Table.Head class="text-right">Avg Latency</Table.Head>
							<Table.Head class="text-right">Last Seen</Table.Head>
						</Table.Row>
					</Table.Header>
					<Table.Body>
						{#each sources as source (source.service_name)}
							{@const errorRate = source.span_count > 0
								? (source.error_count / source.span_count) * 100
								: 0}
							<Table.Row>
								<Table.Cell>
									<div class="flex items-center gap-2">
										<div class="size-2 rounded-full bg-emerald-500"></div>
										<span class="font-medium">{source.service_name}</span>
									</div>
								</Table.Cell>
								<Table.Cell class="text-right font-mono">
									{formatCount(source.span_count)}
								</Table.Cell>
								<Table.Cell class="text-right font-mono">
									{#if source.error_count > 0}
										<span class="text-red-600">{formatCount(source.error_count)}</span>
									{:else}
										<span class="text-muted-foreground">0</span>
									{/if}
								</Table.Cell>
								<Table.Cell class="text-right">
									<Badge variant={getErrorBadgeVariant(errorRate)} class="font-mono text-xs">
										{errorRate.toFixed(2)}%
									</Badge>
								</Table.Cell>
								<Table.Cell class="text-right font-mono">
									{formatDuration(source.avg_duration_ns)}
								</Table.Cell>
								<Table.Cell class="text-right text-muted-foreground">
									{formatRelativeTime(source.last_seen)}
								</Table.Cell>
							</Table.Row>
						{/each}
					</Table.Body>
				</Table.Root>
			</div>
		{/if}
	</div>
</PageContainer>
