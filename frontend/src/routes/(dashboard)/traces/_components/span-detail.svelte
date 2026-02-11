<script lang="ts">
	import type { Span } from '$lib/api/model';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import * as Tabs from '$lib/components/ui/tabs/index.js';
	import XIcon from '@lucide/svelte/icons/x';
	import ExternalLinkIcon from '@lucide/svelte/icons/external-link';
	import CopyIcon from '@lucide/svelte/icons/copy';

	let {
		span,
		onClose = () => {}
	}: {
		span: Span;
		onClose?: () => void;
	} = $props();

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

	function spanKindLabel(kind: number): string {
		switch (kind) {
			case 1:
				return 'Internal';
			case 2:
				return 'Server';
			case 3:
				return 'Client';
			case 4:
				return 'Producer';
			case 5:
				return 'Consumer';
			default:
				return 'Unspecified';
		}
	}

	function statusLabel(code: number): string {
		switch (code) {
			case 1:
				return 'Ok';
			case 2:
				return 'Error';
			default:
				return 'Unset';
		}
	}

	function statusBadgeClass(code: number): string {
		switch (code) {
			case 1:
				return 'bg-green-500/10 text-green-600 border-green-300';
			case 2:
				return 'bg-red-500/10 text-red-600 border-red-300';
			default:
				return 'bg-gray-500/10 text-gray-600 border-gray-300';
		}
	}

	function copyToClipboard(text: string) {
		navigator.clipboard.writeText(text);
	}

	// Build merged attribute list for the Attributes tab
	const allAttributes = $derived.by(() => {
		const sections: { title: string; attrs: Record<string, string> }[] = [];

		if (span.resource_attributes && Object.keys(span.resource_attributes).length > 0) {
			sections.push({ title: 'Resource Attributes', attrs: span.resource_attributes });
		}
		if (span.scope_attributes && Object.keys(span.scope_attributes).length > 0) {
			sections.push({ title: 'Scope Attributes', attrs: span.scope_attributes });
		}
		if (span.span_attributes && Object.keys(span.span_attributes).length > 0) {
			sections.push({ title: 'Span Attributes', attrs: span.span_attributes });
		}

		return sections;
	});

	const events = $derived(span.events ?? []);
	const links = $derived(span.links ?? []);

	// Build "Go to related logs" link
	const relatedLogsUrl = $derived.by(() => {
		let url = `/logs?search=trace_id:${span.trace_id}`;
		return url;
	});
</script>

<div class="flex h-full flex-col border-l bg-card">
	<!-- Header -->
	<div class="flex items-center justify-between border-b px-4 py-3">
		<h3 class="text-sm font-semibold">Span Details</h3>
		<Button variant="ghost" size="icon" class="size-7" onclick={onClose}>
			<XIcon class="size-4" />
		</Button>
	</div>

	<!-- Span Overview -->
	<div class="space-y-3 overflow-auto p-4">
		<!-- Span Name -->
		<div>
			<div class="mb-1 text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
				Span Name
			</div>
			<Badge variant="secondary" class="text-sm font-medium">
				{span.name}
			</Badge>
		</div>

		<!-- Key details grid -->
		<div class="grid grid-cols-2 gap-3">
			<div>
				<div class="mb-0.5 text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
					Span ID
				</div>
				<div class="flex items-center gap-1">
					<code class="text-xs">{span.span_id.slice(0, 8)}...</code>
					<button
						class="text-muted-foreground hover:text-foreground"
						onclick={() => copyToClipboard(span.span_id)}
					>
						<CopyIcon class="size-3" />
					</button>
				</div>
			</div>

			<div>
				<div class="mb-0.5 text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
					Duration
				</div>
				<div class="text-sm font-semibold">{formatDuration(span.duration_ns)}</div>
			</div>

			<div>
				<div class="mb-0.5 text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
					Service
				</div>
				<div class="text-sm">{span.service_name}</div>
			</div>

			<div>
				<div class="mb-0.5 text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
					Start Time
				</div>
				<div class="text-xs">{formatTimestamp(span.start_time)}</div>
			</div>

			<div>
				<div class="mb-0.5 text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
					Kind
				</div>
				<Badge variant="outline" class="text-xs">{spanKindLabel(span.kind)}</Badge>
			</div>

			<div>
				<div class="mb-0.5 text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
					Status
				</div>
				<Badge variant="outline" class={statusBadgeClass(span.status_code)}>
					{statusLabel(span.status_code)}
				</Badge>
			</div>
		</div>

		{#if span.status_message}
			<div>
				<div class="mb-0.5 text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
					Status Message
				</div>
				<div class="rounded bg-red-500/5 px-2 py-1 text-xs text-red-600">
					{span.status_message}
				</div>
			</div>
		{/if}

		<!-- Go to related logs button -->
		<Button variant="outline" size="sm" class="w-full" href={relatedLogsUrl}>
			<ExternalLinkIcon class="mr-2 size-4" />
			Go to related logs
		</Button>

		<!-- Tabs: Attributes, Events, Links -->
		<Tabs.Root value="attributes">
			<Tabs.List class="w-full">
				<Tabs.Trigger value="attributes" class="flex-1">
					Attributes
				</Tabs.Trigger>
				<Tabs.Trigger value="events" class="flex-1">
					Events ({events.length})
				</Tabs.Trigger>
				<Tabs.Trigger value="links" class="flex-1">
					Links ({links.length})
				</Tabs.Trigger>
			</Tabs.List>

			<!-- Attributes Tab -->
			<Tabs.Content value="attributes" class="mt-2">
				{#each allAttributes as section (section.title)}
					<div class="mb-3">
						<div
							class="mb-1 text-[10px] font-medium uppercase tracking-wider text-muted-foreground"
						>
							{section.title}
						</div>
						<div class="divide-y rounded border">
							{#each Object.entries(section.attrs) as [key, value] (key)}
								<div class="flex gap-2 px-2 py-1.5 text-xs">
									<span class="shrink-0 font-medium text-muted-foreground">{key}</span>
									<span class="min-w-0 break-all text-foreground">{value}</span>
								</div>
							{/each}
						</div>
					</div>
				{/each}
				{#if allAttributes.length === 0}
					<div class="py-4 text-center text-xs text-muted-foreground">No attributes</div>
				{/if}
			</Tabs.Content>

			<!-- Events Tab -->
			<Tabs.Content value="events" class="mt-2">
				{#if events.length === 0}
					<div class="py-4 text-center text-xs text-muted-foreground">No events</div>
				{:else}
					<div class="space-y-2">
						{#each events as event, i (i)}
							<div class="rounded border p-2">
								<div class="flex items-center justify-between">
									<span class="text-xs font-medium">{event.name}</span>
									<span class="text-[10px] text-muted-foreground">
										{formatTimestamp(event.timestamp)}
									</span>
								</div>
								{#if event.attributes && Object.keys(event.attributes).length > 0}
									<div class="mt-1 divide-y rounded border">
										{#each Object.entries(event.attributes) as [key, value] (key)}
											<div class="flex gap-2 px-2 py-1 text-xs">
												<span class="shrink-0 font-medium text-muted-foreground">{key}</span>
												<span class="min-w-0 break-all">{value}</span>
											</div>
										{/each}
									</div>
								{/if}
							</div>
						{/each}
					</div>
				{/if}
			</Tabs.Content>

			<!-- Links Tab -->
			<Tabs.Content value="links" class="mt-2">
				{#if links.length === 0}
					<div class="py-4 text-center text-xs text-muted-foreground">No links</div>
				{:else}
					<div class="space-y-2">
						{#each links as link, i (i)}
							<a
								href="/traces/{link.trace_id}"
								class="block rounded border p-2 transition-colors hover:bg-muted/50"
							>
								<div class="flex items-center gap-2">
									<ExternalLinkIcon class="size-3 text-muted-foreground" />
									<span class="font-mono text-xs">{link.trace_id.slice(0, 16)}...</span>
								</div>
								<div class="mt-0.5 font-mono text-[10px] text-muted-foreground">
									span: {link.span_id}
								</div>
								{#if link.attributes && Object.keys(link.attributes).length > 0}
									<div class="mt-1 divide-y rounded border">
										{#each Object.entries(link.attributes) as [key, value] (key)}
											<div class="flex gap-2 px-2 py-1 text-xs">
												<span class="shrink-0 font-medium text-muted-foreground">{key}</span>
												<span class="min-w-0 break-all">{value}</span>
											</div>
										{/each}
									</div>
								{/if}
							</a>
						{/each}
					</div>
				{/if}
			</Tabs.Content>
		</Tabs.Root>
	</div>
</div>
