<script lang="ts">
	import type { MetricMeta } from '$lib/api/model';
	import { Badge } from '$lib/components/ui/badge/index.js';

	let {
		metrics,
		loading,
		selected,
		onSelect
	}: {
		metrics: MetricMeta[];
		loading: boolean;
		selected: string | undefined;
		onSelect: (name: string) => void;
	} = $props();
</script>

<div class="flex-1 overflow-y-auto">
	{#if loading}
		<div class="flex items-center justify-center py-8">
			<p class="text-sm text-muted-foreground">Loading metrics...</p>
		</div>
	{:else if metrics.length === 0}
		<div class="flex items-center justify-center py-8">
			<p class="text-sm text-muted-foreground">No metrics found</p>
		</div>
	{:else}
		<div class="flex flex-col gap-0.5">
			{#each metrics as metric}
				<button
					class="flex flex-col gap-1 rounded-md px-3 py-2 text-left text-sm transition-colors hover:bg-accent {selected === metric.name ? 'bg-accent' : ''}"
					onclick={() => onSelect(metric.name)}
				>
					<div class="flex items-center gap-2">
						<span class="truncate font-medium">{metric.name}</span>
						<Badge variant="outline" class="shrink-0 text-[10px]">{metric.type}</Badge>
					</div>
					{#if metric.description}
						<span class="truncate text-xs text-muted-foreground">{metric.description}</span>
					{/if}
					{#if metric.service_name}
						<span class="text-xs text-muted-foreground">{metric.service_name}</span>
					{/if}
				</button>
			{/each}
		</div>
	{/if}
</div>
