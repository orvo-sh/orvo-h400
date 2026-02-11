<script lang="ts">
	import { Badge } from '$lib/components/ui/badge/index.js';
	import type { TraceSummary } from '$lib/api/model';
	import ClockIcon from '@lucide/svelte/icons/clock';
	import AlertCircleIcon from '@lucide/svelte/icons/circle-alert';
	import LayersIcon from '@lucide/svelte/icons/layers';

	let {
		trace,
		onclick = () => {}
	}: {
		trace: TraceSummary;
		onclick?: () => void;
	} = $props();

	function formatTimestamp(iso: string): string {
		const date = new Date(iso);
		const dateStr = date
			.toLocaleDateString('en-US', {
				month: '2-digit',
				day: '2-digit',
				year: 'numeric'
			})
			.replace(/\//g, '-');
		const timeStr = date.toLocaleTimeString('en-US', {
			hour: '2-digit',
			minute: '2-digit',
			second: '2-digit',
			fractionalSecondDigits: 3,
			hour12: false
		});
		return `${dateStr} ${timeStr}`;
	}

	function formatDuration(ns: number): string {
		const ms = ns / 1_000_000;
		if (ms < 1) return `${(ns / 1_000).toFixed(0)}us`;
		if (ms < 1000) return `${ms.toFixed(1)}ms`;
		return `${(ms / 1000).toFixed(2)}s`;
	}

	const hasErrors = $derived(trace.error_count > 0);

	// Color-code the duration bar proportionally (max 500ms = full width for visual reference)
	const durationMs = $derived(trace.duration_ns / 1_000_000);
	const barWidth = $derived(Math.min(100, Math.max(2, (durationMs / 500) * 100)));
</script>

<button
	class="flex w-full items-center gap-4 border-b px-4 py-3 text-left transition-colors hover:bg-muted/50"
	{onclick}
>
	<!-- Timestamp -->
	<div class="w-[180px] shrink-0 font-mono text-xs text-muted-foreground">
		{formatTimestamp(trace.start_time)}
	</div>

	<!-- Service badge -->
	<div class="w-[130px] shrink-0">
		<Badge variant="outline" class="truncate text-xs font-medium">
			{trace.root_service || 'unknown'}
		</Badge>
	</div>

	<!-- Root span name -->
	<div class="min-w-0 flex-1 truncate text-sm font-medium">
		{trace.root_span_name || 'unknown'}
	</div>

	<!-- Duration bar -->
	<div class="w-[140px] shrink-0">
		<div class="flex items-center gap-2">
			<div class="h-2 flex-1 rounded-full bg-muted">
				<div
					class="h-full rounded-full {hasErrors ? 'bg-red-500' : 'bg-primary'}"
					style="width: {barWidth}%"
				></div>
			</div>
			<span class="w-[60px] text-right font-mono text-xs text-muted-foreground">
				{formatDuration(trace.duration_ns)}
			</span>
		</div>
	</div>

	<!-- Span count -->
	<div class="flex w-[60px] shrink-0 items-center gap-1 text-xs text-muted-foreground">
		<LayersIcon class="size-3" />
		{trace.span_count}
	</div>

	<!-- Error count -->
	<div class="w-[60px] shrink-0">
		{#if hasErrors}
			<div class="flex items-center gap-1 text-xs text-red-500">
				<AlertCircleIcon class="size-3" />
				{trace.error_count}
			</div>
		{/if}
	</div>
</button>
