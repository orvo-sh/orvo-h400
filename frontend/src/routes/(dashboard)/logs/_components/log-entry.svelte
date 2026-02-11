<script lang="ts">
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import MinusIcon from '@lucide/svelte/icons/minus';
	import CopyIcon from '@lucide/svelte/icons/copy';
	import LinkIcon from '@lucide/svelte/icons/link';
	import ClockIcon from '@lucide/svelte/icons/clock';
	import type { LogEntry, LogLevel } from './mock-data';

	function copyAsJson(log: LogEntry) {
		navigator.clipboard.writeText(JSON.stringify(log, null, 2));
	}

	function copyLink(log: LogEntry) {
		const url = `${window.location.origin}/logs?id=${log.id}`;
		navigator.clipboard.writeText(url);
	}

	let {
		log,
		expanded = false,
		onToggle = () => {}
	}: {
		log: LogEntry;
		expanded?: boolean;
		onToggle?: () => void;
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

	function getLevelColor(level: LogLevel): string {
		switch (level) {
			case 'debug':
				return 'text-gray-500';
			case 'info':
				return 'text-blue-500';
			case 'warn':
				return 'text-yellow-500';
			case 'error':
				return 'text-red-500';
			case 'fatal':
				return 'text-red-700 font-bold';
			default:
				return 'text-foreground';
		}
	}

	function getLevelBgColor(level: LogLevel): string {
		switch (level) {
			case 'debug':
				return 'bg-gray-500/10 text-gray-600 border-gray-300';
			case 'info':
				return 'bg-blue-500/10 text-blue-600 border-blue-300';
			case 'warn':
				return 'bg-yellow-500/10 text-yellow-600 border-yellow-300';
			case 'error':
				return 'bg-red-500/10 text-red-600 border-red-300';
			case 'fatal':
				return 'bg-red-700/20 text-red-700 border-red-500';
			default:
				return '';
		}
	}
</script>

<div class="font-mono text-sm">
	<!-- Main log line -->
	<button
		class="flex w-full items-start gap-2 rounded px-2 py-1 text-left transition-colors hover:bg-muted/50"
		onclick={onToggle}
	>
		<!-- Expand/Collapse icon -->
		<span class="mt-0.5 shrink-0 text-muted-foreground">
			{#if expanded}
				<MinusIcon class="size-3" />
			{:else}
				<PlusIcon class="size-3" />
			{/if}
		</span>

		<!-- Log content -->
		<div class="min-w-0 flex-1">
			<span class="text-muted-foreground">{formatTimestamp(log.timestamp)}</span>
			{' '}
			<span class="text-cyan-600">[{log.service}]</span>
			{' '}
			<span class="text-purple-600">[{log.host}]</span>
			{' '}
			<span class={getLevelColor(log.level)}>[{log.level.toUpperCase()}]</span>
			{#if log.group}
				{' '}
				<span class="text-orange-600">[{log.group}]</span>
			{/if}
			{#if log.tags && log.tags.length > 0}
				{#each log.tags as tag (tag)}
					{' '}
					<span class="text-green-600">[{tag}]</span>
				{/each}
			{/if}
			{' '}
			<span class="text-foreground">{log.message}</span>
		</div>
	</button>

	<!-- Expanded details -->
	{#if expanded}
		<div class="mt-2 mb-3 ml-7 overflow-hidden rounded-lg border bg-card">
			<table class="w-full text-sm">
				<tbody>
					<tr class="border-b">
						<td class="w-32 px-4 py-2 font-medium text-muted-foreground">timestamp</td>
						<td class="px-4 py-2">{log.timestamp}</td>
					</tr>
					<tr class="border-b">
						<td class="px-4 py-2 font-medium text-muted-foreground">source</td>
						<td class="px-4 py-2">{log.source}</td>
					</tr>
					<tr class="border-b">
						<td class="px-4 py-2 font-medium text-muted-foreground">service</td>
						<td class="px-4 py-2">{log.service}</td>
					</tr>
					<tr class="border-b">
						<td class="px-4 py-2 font-medium text-muted-foreground">host</td>
						<td class="px-4 py-2">{log.host}</td>
					</tr>
					<tr class="border-b">
						<td class="px-4 py-2 font-medium text-muted-foreground">level</td>
						<td class="px-4 py-2">
							<Badge variant="outline" class={getLevelBgColor(log.level)}>
								{log.level.toUpperCase()}
							</Badge>
						</td>
					</tr>
					{#if log.group}
						<tr class="border-b">
							<td class="px-4 py-2 font-medium text-muted-foreground">group</td>
							<td class="px-4 py-2">{log.group}</td>
						</tr>
					{/if}
					<tr class="border-b">
						<td class="px-4 py-2 font-medium text-muted-foreground">message</td>
						<td class="px-4 py-2">{log.message}</td>
					</tr>
					{#if log.metadata}
						{#each Object.entries(log.metadata) as [key, value] (key)}
							<tr class="border-b last:border-b-0">
								<td class="px-4 py-2 font-medium text-muted-foreground">{key}</td>
								<td class="px-4 py-2">{value}</td>
							</tr>
						{/each}
					{/if}
				</tbody>
			</table>
			<!-- Action buttons -->
			<div class="flex items-center gap-2 border-t bg-muted/30 px-4 py-3">
				<Button variant="outline" size="sm" onclick={() => copyAsJson(log)}>
					<CopyIcon class="mr-1 size-4" />
					Copy as JSON
				</Button>
				<Button variant="outline" size="sm" onclick={() => copyLink(log)}>
					<LinkIcon class="mr-1 size-4" />
					Copy link to log line
				</Button>
				<Button variant="outline" size="sm">
					<ClockIcon class="mr-1 size-4" />
					Open time detective
				</Button>
			</div>
		</div>
	{/if}
</div>
