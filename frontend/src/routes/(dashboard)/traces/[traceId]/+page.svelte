<script lang="ts">
	import { page } from '$app/state';
	import PageContainer from '../../_components/page-container/page-container.svelte';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import ArrowLeftIcon from '@lucide/svelte/icons/arrow-left';
	import LayersIcon from '@lucide/svelte/icons/layers';
	import AlertCircleIcon from '@lucide/svelte/icons/circle-alert';
	import ClockIcon from '@lucide/svelte/icons/clock';
	import CopyIcon from '@lucide/svelte/icons/copy';
	import TraceWaterfall from '../_components/trace-waterfall.svelte';
	import SpanDetail from '../_components/span-detail.svelte';
	import { createGetTrace } from '$lib/api/endpoints/traces/traces';
	import { sessionStore } from '$lib/stores/session';
	import type { Span } from '$lib/api/model';

	const traceId = $derived(page.params.traceId ?? '');
	const traceBreadcrumb = $derived(traceId ? traceId.slice(0, 8) + '...' : 'Trace');
	const orgId = $derived($sessionStore?.active_organization?.id ?? '');

	const traceQuery = createGetTrace(
		() => orgId,
		() => traceId
	);

	const spans = $derived.by((): Span[] => {
		const resp = traceQuery.data;
		if (resp && resp.status === 200) {
			return resp.data.spans ?? [];
		}
		return [];
	});

	// Compute trace-level summary
	const traceSummary = $derived.by(() => {
		if (spans.length === 0) return null;

		let minStart = Infinity;
		let maxEnd = -Infinity;
		let errorCount = 0;
		let rootSpan: Span | null = null;

		for (const s of spans) {
			const start = new Date(s.start_time).getTime();
			const end = new Date(s.end_time).getTime();
			if (start < minStart) minStart = start;
			if (end > maxEnd) maxEnd = end;
			if (s.status_code === 2) errorCount++;
			if (!s.parent_span_id || s.parent_span_id === '0000000000000000') {
				rootSpan = s;
			}
		}

		const durationNs = (maxEnd - minStart) * 1_000_000;
		return {
			rootSpanName: rootSpan?.name ?? spans[0].name,
			rootService: rootSpan?.service_name ?? spans[0].service_name,
			startTime: new Date(minStart).toISOString(),
			durationNs,
			spanCount: spans.length,
			errorCount
		};
	});

	let selectedSpan = $state<Span | null>(null);

	function formatTimestamp(iso: string): string {
		const date = new Date(iso);
		return date.toLocaleString('en-US', {
			month: '2-digit',
			day: '2-digit',
			year: 'numeric',
			hour: '2-digit',
			minute: '2-digit',
			second: '2-digit',
			fractionalSecondDigits: 3,
			hour12: false
		});
	}

	function formatDuration(ns: number): string {
		const ms = ns / 1_000_000;
		if (ms < 1) return `${(ns / 1_000).toFixed(0)}us`;
		if (ms < 1000) return `${ms.toFixed(1)}ms`;
		return `${(ms / 1000).toFixed(2)}s`;
	}

	function copyTraceId() {
		if (!traceId) return;
		navigator.clipboard.writeText(traceId);
	}
</script>

<PageContainer
	breadcrumbs={[
		{ title: 'Traces', href: '/traces' },
		{ title: traceBreadcrumb }
	]}
>
	{#if traceQuery.isPending}
		<div class="flex h-64 items-center justify-center text-muted-foreground">
			Loading trace...
		</div>
	{:else if spans.length === 0}
		<div class="flex h-64 flex-col items-center justify-center gap-2 text-muted-foreground">
			<p>No spans found for this trace</p>
			<Button variant="outline" href="/traces">
				<ArrowLeftIcon class="mr-2 size-4" />
				Back to traces
			</Button>
		</div>
	{:else if traceSummary}
		<div class="flex h-full flex-col gap-4">
			<!-- Header -->
			<div class="flex items-start justify-between">
				<div class="flex items-start gap-3">
					<Button variant="ghost" size="icon" href="/traces" class="mt-0.5">
						<ArrowLeftIcon class="size-5" />
					</Button>
					<div>
						<div class="flex items-center gap-2">
							<h1 class="text-lg font-semibold">
								{traceSummary.rootService}
							</h1>
							<span class="text-muted-foreground">/</span>
							<span class="text-lg text-muted-foreground">{traceSummary.rootSpanName}</span>
						</div>
						<div class="mt-1 flex items-center gap-3 text-sm text-muted-foreground">
							<div class="flex items-center gap-1">
								<span class="font-mono text-xs">{traceId.slice(0, 16)}...</span>
								<button class="hover:text-foreground" onclick={copyTraceId}>
									<CopyIcon class="size-3" />
								</button>
							</div>
							<div class="flex items-center gap-1">
								<ClockIcon class="size-3.5" />
								{formatTimestamp(traceSummary.startTime)}
							</div>
						</div>
					</div>
				</div>

				<!-- Summary badges -->
				<div class="flex items-center gap-3">
					<div class="flex items-center gap-1.5 text-sm">
						<ClockIcon class="size-4 text-muted-foreground" />
						<span class="font-semibold">{formatDuration(traceSummary.durationNs)}</span>
					</div>
					<Badge variant="outline" class="gap-1">
						<LayersIcon class="size-3" />
						{traceSummary.spanCount} spans
					</Badge>
					{#if traceSummary.errorCount > 0}
						<Badge variant="destructive" class="gap-1">
							<AlertCircleIcon class="size-3" />
							{traceSummary.errorCount} errors
						</Badge>
					{/if}
				</div>
			</div>

			<!-- Main content: Waterfall + Span Detail sidebar -->
			<div class="flex min-h-0 flex-1 gap-0">
				<!-- Waterfall -->
				<div class="min-w-0 flex-1 overflow-auto">
					<TraceWaterfall
						{spans}
						selectedSpanId={selectedSpan?.span_id}
						onSelectSpan={(s) => (selectedSpan = s)}
					/>
				</div>

				<!-- Span detail sidebar -->
				{#if selectedSpan}
					<div class="w-[380px] shrink-0 overflow-auto">
						<SpanDetail span={selectedSpan} onClose={() => (selectedSpan = null)} />
					</div>
				{/if}
			</div>
		</div>
	{/if}
</PageContainer>
