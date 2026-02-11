<script lang="ts">
	import { Button } from '$lib/components/ui/button/index.js';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu/index.js';
	import ColumnsIcon from '@lucide/svelte/icons/columns-3';
	import RowsIcon from '@lucide/svelte/icons/rows-3';
	import PlayIcon from '@lucide/svelte/icons/play';
	import PauseIcon from '@lucide/svelte/icons/pause';
	import CopyIcon from '@lucide/svelte/icons/copy';
	import LinkIcon from '@lucide/svelte/icons/link';
	import ClockIcon from '@lucide/svelte/icons/clock';
	import LogEntry from './log-entry.svelte';
	import type { LogEntry as LogEntryType } from './mock-data';

	let {
		logs,
		isLive = false,
		onToggleLive = () => {}
	}: {
		logs: LogEntryType[];
		isLive?: boolean;
		onToggleLive?: () => void;
	} = $props();

	let expandedIds = $state<Set<string>>(new Set());
	let columnsVisible = $state(5);
	let totalColumns = $state(5);
	let rowHeight = $state<'compact' | 'default' | 'expanded'>('default');

	function toggleExpanded(id: string) {
		const newSet = new Set(expandedIds);
		if (newSet.has(id)) {
			newSet.delete(id);
		} else {
			newSet.add(id);
		}
		expandedIds = newSet;
	}

	function copyAsJson(log: LogEntryType) {
		navigator.clipboard.writeText(JSON.stringify(log, null, 2));
	}

	function copyLink(log: LogEntryType) {
		const url = `${window.location.origin}/logs?id=${log.id}`;
		navigator.clipboard.writeText(url);
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
						expanded={expandedIds.has(log.id)}
						onToggle={() => toggleExpanded(log.id)}
					/>
				{/each}
			</div>
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
