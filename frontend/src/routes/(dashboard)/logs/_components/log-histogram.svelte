<script lang="ts">
	import { Button } from '$lib/components/ui/button/index.js';
	import * as Card from '$lib/components/ui/card/index.js';
	import EyeOffIcon from '@lucide/svelte/icons/eye-off';
	import type { HistogramBucket } from '$lib/api/model';

	let {
		data,
		visible = true,
		loading = false,
		onToggleVisibility = () => {}
	}: {
		data: HistogramBucket[];
		visible?: boolean;
		loading?: boolean;
		onToggleVisibility?: () => void;
	} = $props();

	const maxCount = $derived(Math.max(...data.map((d) => d.count), 1));

	function formatTime(iso: string): string {
		const date = new Date(iso);
		return date.toLocaleTimeString('en-US', {
			hour: '2-digit',
			minute: '2-digit',
			hour12: false
		});
	}

	// Get every 4th label for x-axis
	const xLabels = $derived(data.filter((_, i) => i % 4 === 0).map((d) => formatTime(d.time)));
</script>

{#if visible}
	<Card.Root class="border-0 bg-muted/30">
		<Card.Header class="flex flex-row items-center justify-between px-4 py-3">
			<Card.Title class="text-sm font-medium">All logs</Card.Title>
			<Button variant="ghost" size="sm" onclick={onToggleVisibility}>
				<EyeOffIcon class="mr-1 size-4" />
				Hide chart
			</Button>
		</Card.Header>
		<Card.Content class="px-4 pb-4">
			<!-- Chart Area -->
			<div class="h-20">
				{#if loading && data.length === 0}
					<!-- Loading skeleton -->
					<div class="flex h-full items-end gap-[2px] pl-10">
						{#each Array(24) as _, i (i)}
							<div
								class="flex-1 animate-pulse rounded-t-sm bg-muted"
								style="height: {20 + Math.random() * 60}%"
							></div>
						{/each}
					</div>
				{:else if data.length === 0}
					<div class="flex h-full items-center justify-center text-sm text-muted-foreground">
						No histogram data
					</div>
				{:else}
				<div class="flex h-full">
					<div
						class="flex w-8 flex-col justify-between pr-2 text-right text-xs text-muted-foreground"
					>
						<span>{maxCount}</span>
						<span>{Math.round(maxCount / 2)}</span>
						<span>0</span>
					</div>

					<!-- Bars -->
					<div class="flex flex-1 items-end gap-[2px]">
						{#each data as bucket, i (i)}
							{@const height = (bucket.count / maxCount) * 100}
							<div
								class="min-h-[2px] flex-1 cursor-pointer rounded-t-sm bg-primary/70 transition-colors hover:bg-primary"
								style="height: {height}%"
								title={`${formatTime(bucket.time)}: ${bucket.count} logs`}
							></div>
						{/each}
					</div>
				</div>

				<!-- X-axis labels -->
				<div class="mt-1 flex pl-10">
					<div class="flex flex-1 justify-between text-xs text-muted-foreground">
						{#each xLabels as label, i (i)}
							<span>{label}</span>
						{/each}
					</div>
				</div>
				{/if}
			</div>
		</Card.Content>
	</Card.Root>
{:else}
	<div class="flex items-center justify-between rounded-lg bg-muted/30 px-4 py-2">
		<span class="text-sm text-muted-foreground">Chart hidden</span>
		<Button variant="ghost" size="sm" onclick={onToggleVisibility}>Show chart</Button>
	</div>
{/if}
