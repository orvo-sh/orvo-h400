<script lang="ts">
	import type { TraceSummary } from '$lib/api/model';
	import { Button } from '$lib/components/ui/button/index.js';
	import TraceEntry from './trace-entry.svelte';

	let {
		traces,
		onTraceClick = (_traceId: string) => {},
		onLoadMore = undefined,
		hasMore = false,
		loading = false
	}: {
		traces: TraceSummary[];
		onTraceClick?: (traceId: string) => void;
		onLoadMore?: (() => void) | undefined;
		hasMore?: boolean;
		loading?: boolean;
	} = $props();
</script>

<div class="flex h-full flex-col">
	<div class="flex-1 overflow-auto rounded-lg border bg-card">
		<!-- Column headers -->
		<div
			class="flex items-center gap-4 border-b bg-muted/30 px-4 py-2 text-xs font-medium text-muted-foreground"
		>
			<div class="w-[180px] shrink-0">Timestamp</div>
			<div class="w-[130px] shrink-0">Service</div>
			<div class="min-w-0 flex-1">Root Span</div>
			<div class="w-[140px] shrink-0">Duration</div>
			<div class="w-[60px] shrink-0">Spans</div>
			<div class="w-[60px] shrink-0">Errors</div>
		</div>

		{#if traces.length === 0 && !loading}
			<div class="flex h-40 items-center justify-center text-muted-foreground">
				No traces found matching your filters
			</div>
		{:else}
			{#each traces as trace (trace.trace_id)}
				<TraceEntry {trace} onclick={() => onTraceClick(trace.trace_id)} />
			{/each}

			{#if hasMore && onLoadMore}
				<div class="flex justify-center border-t py-3">
					<Button variant="outline" size="sm" onclick={onLoadMore} disabled={loading}>
						{#if loading}
							Loading...
						{:else}
							Load more
						{/if}
					</Button>
				</div>
			{/if}
		{/if}

		{#if loading && traces.length === 0}
			<div class="flex h-40 items-center justify-center text-muted-foreground">
				Loading traces...
			</div>
		{/if}
	</div>
</div>
