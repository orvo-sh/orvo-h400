<script lang="ts">
	import type { LogRecord } from '$lib/api/model';
	import { Button } from '$lib/components/ui/button/index.js';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu/index.js';
	import ColumnsIcon from '@lucide/svelte/icons/columns-3';
	import PauseIcon from '@lucide/svelte/icons/pause';
	import PlayIcon from '@lucide/svelte/icons/play';
	import RowsIcon from '@lucide/svelte/icons/rows-3';
	import LogEntry from './log-entry.svelte';

	let {
		logs,
		isLive = false,
		onToggleLive = () => {},
		onLoadMore = undefined,
		hasMore = false,
		loading = false,
		onAutoResolve = undefined,
		autoResolveBusyLogID = undefined
	}: {
		logs: LogRecord[];
		isLive?: boolean;
		onToggleLive?: () => void;
		onLoadMore?: (() => void) | undefined;
		hasMore?: boolean;
		loading?: boolean;
		onAutoResolve?: ((log: LogRecord) => void) | undefined;
		autoResolveBusyLogID?: string | undefined;
	} = $props();

	let expandedTimestamps = $state<Set<string>>(new Set());
	let columnsVisible = $state(5);
	let totalColumns = $state(5);
	let rowHeight = $state<'compact' | 'default' | 'expanded'>('default');

	function toggleExpanded(timestamp: string) {
		const newSet = new Set(expandedTimestamps);
		if (newSet.has(timestamp)) {
			newSet.delete(timestamp);
		} else {
			newSet.add(timestamp);
		}
		expandedTimestamps = newSet;
	}
</script>

<div class="flex h-full flex-col">
	<!-- Log entries -->
	<div class="flex-1 overflow-auto rounded-lg border bg-card">
		{#if logs.length === 0}
			<div class="flex h-40 items-center justify-center text-muted-foreground">
				No logs found matching your filters
			</div>
		{:else}
			<div class="divide-y">
				{#each logs as log (log.id)}
					<LogEntry
						{log}
						expanded={expandedTimestamps.has(log.timestamp)}
						onToggle={() => toggleExpanded(log.timestamp)}
						onAutoResolve={() => onAutoResolve?.(log)}
						autoResolveBusy={autoResolveBusyLogID === log.id}
					/>
				{/each}
			</div>
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
	</div>

	<!-- Bottom toolbar -->
	<div class="mt-3 flex items-center gap-2 border-t pt-3">
		<!-- Columns selector -->
		<DropdownMenu.Root>
			<DropdownMenu.Trigger>
				{#snippet child({ props })}
					<Button variant="outline" size="sm" {...props}>
						<ColumnsIcon class="mr-1 size-4" />
						Columns {columnsVisible} / {totalColumns}
					</Button>
				{/snippet}
			</DropdownMenu.Trigger>
			<DropdownMenu.Content>
				<DropdownMenu.CheckboxItem checked={true}>Timestamp</DropdownMenu.CheckboxItem>
				<DropdownMenu.CheckboxItem checked={true}>Service</DropdownMenu.CheckboxItem>
				<DropdownMenu.CheckboxItem checked={true}>Host</DropdownMenu.CheckboxItem>
				<DropdownMenu.CheckboxItem checked={true}>Level</DropdownMenu.CheckboxItem>
				<DropdownMenu.CheckboxItem checked={true}>Message</DropdownMenu.CheckboxItem>
			</DropdownMenu.Content>
		</DropdownMenu.Root>

		<!-- Row height selector -->
		<DropdownMenu.Root>
			<DropdownMenu.Trigger>
				{#snippet child({ props })}
					<Button variant="outline" size="sm" {...props}>
						<RowsIcon class="mr-1 size-4" />
						Row height
					</Button>
				{/snippet}
			</DropdownMenu.Trigger>
			<DropdownMenu.Content>
				<DropdownMenu.RadioGroup bind:value={rowHeight}>
					<DropdownMenu.RadioItem value="compact">Compact</DropdownMenu.RadioItem>
					<DropdownMenu.RadioItem value="default">Default</DropdownMenu.RadioItem>
					<DropdownMenu.RadioItem value="expanded">Expanded</DropdownMenu.RadioItem>
				</DropdownMenu.RadioGroup>
			</DropdownMenu.Content>
		</DropdownMenu.Root>

		<div class="flex-1"></div>

		<!-- Live mode toggle -->
		<Button variant={isLive ? 'default' : 'outline'} size="sm" onclick={onToggleLive}>
			{#if isLive}
				<PauseIcon class="mr-1 size-4" />
				Pause
			{:else}
				<PlayIcon class="mr-1 size-4" />
				Go to live
			{/if}
		</Button>
	</div>
</div>
