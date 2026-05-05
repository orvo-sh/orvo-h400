<script lang="ts">
	import type { Span } from '$lib/api/model';
	import SpanBar from './span-bar.svelte';
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
	import ChevronDownIcon from '@lucide/svelte/icons/chevron-down';

	let {
		spans,
		selectedSpanId = undefined,
		onSelectSpan = (_span: Span) => {}
	}: {
		spans: Span[];
		selectedSpanId?: string;
		onSelectSpan?: (span: Span) => void;
	} = $props();

	const SLOW_SPAN_THRESHOLD_NS = 3_000_000_000;

	// Build tree structure from flat span list
	interface SpanNode {
		span: Span;
		children: SpanNode[];
		depth: number;
	}

	// Service color palette
	const SERVICE_COLORS = [
		'bg-blue-500',
		'bg-emerald-500',
		'bg-violet-500',
		'bg-amber-500',
		'bg-cyan-500',
		'bg-pink-500',
		'bg-lime-500',
		'bg-orange-500',
		'bg-teal-500',
		'bg-indigo-500'
	];

	const SERVICE_DOT_COLORS = [
		'bg-blue-500',
		'bg-emerald-500',
		'bg-violet-500',
		'bg-amber-500',
		'bg-cyan-500',
		'bg-pink-500',
		'bg-lime-500',
		'bg-orange-500',
		'bg-teal-500',
		'bg-indigo-500'
	];

	// Compute service color map
	const serviceColorMap = $derived.by(() => {
		const map = new Map<string, number>();
		let idx = 0;
		for (const s of spans) {
			if (!map.has(s.service_name)) {
				map.set(s.service_name, idx);
				idx++;
			}
		}
		return map;
	});

	function getServiceColor(service: string): string {
		const idx = serviceColorMap.get(service) ?? 0;
		return SERVICE_COLORS[idx % SERVICE_COLORS.length];
	}

	function getServiceDotColor(service: string): string {
		const idx = serviceColorMap.get(service) ?? 0;
		return SERVICE_DOT_COLORS[idx % SERVICE_DOT_COLORS.length];
	}

	// Build the span tree
	const tree = $derived.by((): SpanNode[] => {
		if (spans.length === 0) return [];

		const spanMap = new Map<string, SpanNode>();
		const roots: SpanNode[] = [];

		// Create nodes
		for (const s of spans) {
			spanMap.set(s.span_id, { span: s, children: [], depth: 0 });
		}

		// Link parent-child
		for (const node of spanMap.values()) {
			const parentId = node.span.parent_span_id;
			if (parentId && parentId !== '0000000000000000' && spanMap.has(parentId)) {
				spanMap.get(parentId)!.children.push(node);
			} else {
				roots.push(node);
			}
		}

		// Sort children by start_time
		function sortChildren(node: SpanNode) {
			node.children.sort(
				(a, b) => new Date(a.span.start_time).getTime() - new Date(b.span.start_time).getTime()
			);
			for (const child of node.children) {
				sortChildren(child);
			}
		}

		for (const root of roots) {
			sortChildren(root);
		}

		// Flatten tree with depth info
		return roots;
	});

	// Flatten tree for rendering
	let collapsedSpans = $state<Set<string>>(new Set());

	interface FlatRow {
		span: Span;
		depth: number;
		hasChildren: boolean;
		isCollapsed: boolean;
	}

	const flatRows = $derived.by((): FlatRow[] => {
		const rows: FlatRow[] = [];

		function walk(nodes: SpanNode[], depth: number) {
			for (const node of nodes) {
				const isCollapsed = collapsedSpans.has(node.span.span_id);
				rows.push({
					span: node.span,
					depth,
					hasChildren: node.children.length > 0,
					isCollapsed
				});
				if (!isCollapsed) {
					walk(node.children, depth + 1);
				}
			}
		}

		walk(tree, 0);
		return rows;
	});

	// Compute trace time bounds
	const traceBounds = $derived.by(() => {
		if (spans.length === 0) return { start: 0, end: 1, duration: 1 };
		let minStart = Infinity;
		let maxEnd = -Infinity;
		for (const s of spans) {
			const start = new Date(s.start_time).getTime();
			const end = new Date(s.end_time).getTime();
			if (start < minStart) minStart = start;
			if (end > maxEnd) maxEnd = end;
		}
		const duration = Math.max(maxEnd - minStart, 1);
		return { start: minStart, end: maxEnd, duration };
	});

	function formatDuration(ns: number): string {
		const ms = ns / 1_000_000;
		if (ms < 1) return `${(ns / 1_000).toFixed(0)}us`;
		if (ms < 1000) return `${ms.toFixed(1)}ms`;
		return `${(ms / 1000).toFixed(2)}s`;
	}

	function formatRulerLabel(ms: number): string {
		if (ms < 1) return `${(ms * 1000).toFixed(0)}us`;
		if (ms < 1000) return `${ms.toFixed(0)}ms`;
		return `${(ms / 1000).toFixed(1)}s`;
	}

	function toggleCollapse(spanId: string) {
		const newSet = new Set(collapsedSpans);
		if (newSet.has(spanId)) {
			newSet.delete(spanId);
		} else {
			newSet.add(spanId);
		}
		collapsedSpans = newSet;
	}

	// Timeline ruler ticks
	const rulerTicks = $derived.by(() => {
		const totalMs = traceBounds.duration;
		const tickCount = 6;
		const ticks: { label: string; percent: number }[] = [];
		for (let i = 0; i <= tickCount; i++) {
			const ms = (totalMs / tickCount) * i;
			ticks.push({
				label: formatRulerLabel(ms),
				percent: (i / tickCount) * 100
			});
		}
		return ticks;
	});

	// Service legend
	const serviceList = $derived.by(() => {
		return Array.from(serviceColorMap.entries()).map(([name, _idx]) => ({
			name,
			color: getServiceDotColor(name)
		}));
	});
</script>

<div class="flex flex-col rounded-lg border bg-card">
	<!-- Service legend -->
	{#if serviceList.length > 1}
		<div class="flex flex-wrap gap-3 border-b px-4 py-2">
			{#each serviceList as svc (svc.name)}
				<div class="flex items-center gap-1.5 text-xs text-muted-foreground">
					<div class="size-2.5 rounded-full {svc.color}"></div>
					{svc.name}
				</div>
			{/each}
		</div>
	{/if}

	<!-- Timeline ruler -->
	<div class="relative flex h-7 border-b bg-muted/20">
		<div class="w-[320px] shrink-0 border-r"></div>
		<div class="relative flex-1">
			{#each rulerTicks as tick (tick.percent)}
				<div
					class="absolute top-0 bottom-0 border-l border-dashed border-border/40"
					style="left: {tick.percent}%"
				>
					<span class="absolute top-1 left-1 text-[10px] text-muted-foreground">
						{tick.label}
					</span>
				</div>
			{/each}
		</div>
	</div>

	<!-- Span rows -->
	<div class="max-h-[600px] overflow-auto">
		{#each flatRows as row (row.span.span_id)}
			{@const spanStart = new Date(row.span.start_time).getTime()}
			{@const offsetPercent = ((spanStart - traceBounds.start) / traceBounds.duration) * 100}
			{@const widthPercent = (row.span.duration_ns / 1_000_000 / traceBounds.duration) * 100}
			{@const isError = row.span.status_code === 2}
			{@const isSlow = row.span.duration_ns >= SLOW_SPAN_THRESHOLD_NS}
			{@const isSelected = row.span.span_id === selectedSpanId}

				<div
					role="button"
					tabindex="0"
					class="flex w-full items-center transition-colors hover:bg-muted/40 {isSelected
						? 'bg-muted/60'
						: ''}"
					onclick={() => onSelectSpan(row.span)}
					onkeydown={(e) => {
						if (e.key === 'Enter' || e.key === ' ') {
							e.preventDefault();
							onSelectSpan(row.span);
						}
					}}
				>
				<!-- Span name column -->
				<div
					class="flex w-[320px] shrink-0 items-center gap-1 overflow-hidden border-r px-2 py-1.5"
				>
					<!-- Indent -->
					<div style="width: {row.depth * 20}px" class="shrink-0"></div>

					<!-- Expand/collapse toggle -->
					{#if row.hasChildren}
						<button
							class="shrink-0 rounded p-0.5 text-muted-foreground hover:bg-muted"
							onclick={(e) => { e.stopPropagation(); toggleCollapse(row.span.span_id); }}
						>
							{#if row.isCollapsed}
								<ChevronRightIcon class="size-3.5" />
							{:else}
								<ChevronDownIcon class="size-3.5" />
							{/if}
						</button>
					{:else}
						<div class="w-[22px] shrink-0"></div>
					{/if}

					<!-- Service dot -->
					<div class="size-2 shrink-0 rounded-full {getServiceDotColor(row.span.service_name)}">
					</div>

					<!-- Span name -->
					<span class="truncate text-xs font-medium {isError || isSlow ? 'text-red-500' : ''}">
						{row.span.name}
					</span>
				</div>

				<!-- Timing bar column -->
				<div class="flex-1 px-2 py-1">
					<SpanBar
						{offsetPercent}
						{widthPercent}
						color={isError || isSlow ? 'bg-red-500' : getServiceColor(row.span.service_name)}
						isError={isError || isSlow}
						{isSelected}
						label={formatDuration(row.span.duration_ns)}
					/>
				</div>
				</div>
			{/each}
		</div>
	</div>
