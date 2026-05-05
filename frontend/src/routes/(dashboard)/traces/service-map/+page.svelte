<script lang="ts">
	import PageContainer from '../../_components/page-container/page-container.svelte';
	import NetworkIcon from '@lucide/svelte/icons/network';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { Separator } from '$lib/components/ui/separator/index.js';
	import * as Sheet from '$lib/components/ui/sheet/index.js';
	import { createGetServiceMap } from '$lib/api/endpoints/traces/traces';
	import { sessionStore } from '$lib/stores/session';
	import type { ServiceEdge } from '$lib/api/model';

	const orgId = $derived($sessionStore?.active_organization?.id ?? '');
	const serviceMapQuery = createGetServiceMap(() => orgId);

	const GRAPH_WIDTH = 1040;
	const GRAPH_HEIGHT = 680;
	const GRAPH_CENTER_X = GRAPH_WIDTH / 2;
	const GRAPH_CENTER_Y = GRAPH_HEIGHT / 2;
	const GRAPH_PADDING_X = 92;
	const GRAPH_PADDING_Y = 72;
	const NODE_RADIUS = 30;
	const NODE_COLLISION_DISTANCE = NODE_RADIUS * 2 + 42;

	const NODE_COLORS = [
		'#3b82f6', // blue-500
		'#10b981', // emerald-500
		'#8b5cf6', // violet-500
		'#f59e0b', // amber-500
		'#06b6d4', // cyan-500
		'#ec4899', // pink-500
		'#84cc16', // lime-500
		'#f97316', // orange-500
		'#14b8a6', // teal-500
		'#6366f1' // indigo-500
	];

	const edges = $derived.by((): ServiceEdge[] => {
		const resp = serviceMapQuery.data;
		if (resp && resp.status === 200) {
			return resp.data.edges ?? [];
		}
		return [];
	});

	// -- Graph layout types --
	interface GraphNode {
		id: string;
		x: number;
		y: number;
		vx: number;
		vy: number;
		color: string;
		totalRequests: number;
		totalErrors: number;
	}

	interface GraphEdge {
		source: string;
		target: string;
		requestCount: number;
		errorCount: number;
		avgDurationNs: number;
		errorRate: number;
	}

	interface GraphConnection extends GraphEdge {
		counterpart: string;
		direction: 'incoming' | 'outgoing';
	}

	// -- Derive graph data --
	const graphData = $derived.by(() => {
		if (edges.length === 0) return { nodes: [] as GraphNode[], edges: [] as GraphEdge[] };

		// Collect unique nodes
		const nodeMap = new Map<string, { requests: number; errors: number }>();
		const graphEdges: GraphEdge[] = [];

		for (const e of edges) {
			if (!nodeMap.has(e.source)) nodeMap.set(e.source, { requests: 0, errors: 0 });
			if (!nodeMap.has(e.target)) nodeMap.set(e.target, { requests: 0, errors: 0 });

			const src = nodeMap.get(e.source)!;
			src.requests += e.request_count;
			src.errors += e.error_count;

			const tgt = nodeMap.get(e.target)!;
			tgt.requests += e.request_count;
			tgt.errors += e.error_count;

			graphEdges.push({
				source: e.source,
				target: e.target,
				requestCount: e.request_count,
				errorCount: e.error_count,
				avgDurationNs: e.avg_duration_ns,
				errorRate: e.request_count > 0 ? (e.error_count / e.request_count) * 100 : 0
			});
		}

		// Position nodes in a wide ring before relaxing into the force layout.
		const nodeIds = Array.from(nodeMap.keys());
		const radius =
			nodeIds.length === 1
				? 0
				: Math.min(Math.min(GRAPH_WIDTH, GRAPH_HEIGHT) * 0.38, 180 + nodeIds.length * 18);

		const graphNodes: GraphNode[] = nodeIds.map((id, i) => {
			const angle = (2 * Math.PI * i) / nodeIds.length - Math.PI / 2;
			return {
				id,
				x: GRAPH_CENTER_X + radius * Math.cos(angle),
				y: GRAPH_CENTER_Y + radius * Math.sin(angle),
				vx: 0,
				vy: 0,
				color: getNodeColor(i),
				totalRequests: nodeMap.get(id)!.requests,
				totalErrors: nodeMap.get(id)!.errors
			};
		});

		// Run a slightly roomier force-directed simulation with collision spacing.
		simulateForces(graphNodes, graphEdges, 180);

		return { nodes: graphNodes, edges: graphEdges };
	});

	function simulateForces(nodes: GraphNode[], edges: GraphEdge[], iterations: number) {
		const springLength = 180;
		const repulsion = 22000;
		const damping = 0.82;
		const gravityStrength = 0.008;
		const springStrength = 0.05;
		const nodeIndex = new Map(nodes.map((node, index) => [node.id, index]));

		for (let iter = 0; iter < iterations; iter++) {
			// Repulsion between all node pairs
			for (let i = 0; i < nodes.length; i++) {
				for (let j = i + 1; j < nodes.length; j++) {
					let dx = nodes[i].x - nodes[j].x;
					let dy = nodes[i].y - nodes[j].y;
					let dist = Math.sqrt(dx * dx + dy * dy);
					if (dist < 1) dist = 1;
					const force = repulsion / (dist * dist);
					const fx = (dx / dist) * force;
					const fy = (dy / dist) * force;
					nodes[i].vx += fx;
					nodes[i].vy += fy;
					nodes[j].vx -= fx;
					nodes[j].vy -= fy;
				}
			}

			// Spring attraction along edges
			for (const e of edges) {
				const si = nodeIndex.get(e.source);
				const ti = nodeIndex.get(e.target);
				if (si === undefined || ti === undefined) continue;
				const sn = nodes[si];
				const tn = nodes[ti];
				let dx = tn.x - sn.x;
				let dy = tn.y - sn.y;
				let dist = Math.sqrt(dx * dx + dy * dy);
				if (dist < 1) dist = 1;
				const force = (dist - springLength) * springStrength;
				const fx = (dx / dist) * force;
				const fy = (dy / dist) * force;
				sn.vx += fx;
				sn.vy += fy;
				tn.vx -= fx;
				tn.vy -= fy;
			}

			// Gravity toward center
			for (const n of nodes) {
				n.vx += (GRAPH_CENTER_X - n.x) * gravityStrength;
				n.vy += (GRAPH_CENTER_Y - n.y) * gravityStrength;
			}

			// Apply velocities with damping
			for (const n of nodes) {
				n.vx *= damping;
				n.vy *= damping;
				n.x += n.vx;
				n.y += n.vy;
			}

			resolveNodeCollisions(nodes, NODE_COLLISION_DISTANCE);

			for (const n of nodes) {
				n.x = Math.max(GRAPH_PADDING_X, Math.min(GRAPH_WIDTH - GRAPH_PADDING_X, n.x));
				n.y = Math.max(GRAPH_PADDING_Y, Math.min(GRAPH_HEIGHT - GRAPH_PADDING_Y, n.y));
			}
		}
	}

	function resolveNodeCollisions(nodes: GraphNode[], minDistance: number) {
		for (let i = 0; i < nodes.length; i++) {
			for (let j = i + 1; j < nodes.length; j++) {
				let dx = nodes[j].x - nodes[i].x;
				let dy = nodes[j].y - nodes[i].y;
				let dist = Math.sqrt(dx * dx + dy * dy);

				if (dist < 0.001) {
					const angle = ((i + j + 1) * Math.PI) / 6;
					dx = Math.cos(angle);
					dy = Math.sin(angle);
					dist = 1;
				}

				if (dist >= minDistance) continue;

				const overlap = (minDistance - dist) / 2;
				const ux = dx / dist;
				const uy = dy / dist;

				nodes[i].x -= ux * overlap;
				nodes[i].y -= uy * overlap;
				nodes[j].x += ux * overlap;
				nodes[j].y += uy * overlap;
			}
		}
	}

	// -- Helpers --
	function formatDuration(ns: number): string {
		const ms = ns / 1_000_000;
		if (ms < 1) return `${(ns / 1_000).toFixed(0)}us`;
		if (ms < 1000) return `${ms.toFixed(1)}ms`;
		return `${(ms / 1000).toFixed(2)}s`;
	}

	function formatCount(n: number): string {
		if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
		if (n >= 1_000) return `${(n / 1_000).toFixed(1)}k`;
		return n.toString();
	}

	function getEdgeColor(errorRate: number): string {
		if (errorRate === 0) return '#10b981'; // emerald-500
		if (errorRate < 1) return '#f59e0b'; // amber-500
		if (errorRate < 5) return '#f97316'; // orange-500
		return '#ef4444'; // red-500
	}

	function getEdgeStrokeWidth(requestCount: number, maxCount: number): number {
		if (maxCount <= 0) return 1.5;
		const ratio = requestCount / maxCount;
		return 1.5 + ratio * 3;
	}

	const maxRequestCount = $derived(
		graphData.edges.length > 0 ? Math.max(...graphData.edges.map((e) => e.requestCount)) : 1
	);

	function getNodeColor(index: number): string {
		return NODE_COLORS[index % NODE_COLORS.length];
	}

	// -- Hover and selection state --
	let hoveredEdge = $state<GraphEdge | null>(null);
	let hoveredNode = $state<GraphNode | null>(null);
	let selectedNodeId = $state<string | null>(null);
	let tooltipX = $state(0);
	let tooltipY = $state(0);

	const selectedNodeDetails = $derived.by(() => {
		if (!selectedNodeId) return null;

		const node = graphData.nodes.find((graphNode) => graphNode.id === selectedNodeId);
		if (!node) return null;

		const incoming: GraphConnection[] = graphData.edges
			.filter((edge) => edge.target === selectedNodeId)
			.sort((left, right) => right.requestCount - left.requestCount)
			.map((edge) => ({
				...edge,
				counterpart: edge.source,
				direction: 'incoming'
			}));

		const outgoing: GraphConnection[] = graphData.edges
			.filter((edge) => edge.source === selectedNodeId)
			.sort((left, right) => right.requestCount - left.requestCount)
			.map((edge) => ({
				...edge,
				counterpart: edge.target,
				direction: 'outgoing'
			}));

		const incomingRequests = incoming.reduce((sum, edge) => sum + edge.requestCount, 0);
		const outgoingRequests = outgoing.reduce((sum, edge) => sum + edge.requestCount, 0);
		const connectedServices = new Set([
			...incoming.map((edge) => edge.counterpart),
			...outgoing.map((edge) => edge.counterpart)
		]).size;
		const errorRate = node.totalRequests > 0 ? (node.totalErrors / node.totalRequests) * 100 : 0;

		return {
			node,
			incoming,
			outgoing,
			incomingRequests,
			outgoingRequests,
			connectedServices,
			errorRate
		};
	});

	$effect(() => {
		if (selectedNodeId && !graphData.nodes.some((node) => node.id === selectedNodeId)) {
			selectedNodeId = null;
		}
	});

	function handleEdgeMouseEnter(e: MouseEvent, edge: GraphEdge) {
		hoveredEdge = edge;
		hoveredNode = null;
		tooltipX = e.clientX;
		tooltipY = e.clientY;
	}

	function handleNodeMouseEnter(e: MouseEvent, node: GraphNode) {
		hoveredNode = node;
		hoveredEdge = null;
		tooltipX = e.clientX;
		tooltipY = e.clientY;
	}

	function handleNodeSelect(nodeId: string) {
		selectedNodeId = nodeId;
		hoveredEdge = null;
		hoveredNode = null;
	}

	function handleNodeKeydown(event: KeyboardEvent, nodeId: string) {
		if (event.key !== 'Enter' && event.key !== ' ') return;
		event.preventDefault();
		handleNodeSelect(nodeId);
	}

	function handleMouseLeave() {
		hoveredEdge = null;
		hoveredNode = null;
	}

	// Compute edge path with arrowhead offset
	function computeEdgePath(
		sourceNode: GraphNode,
		targetNode: GraphNode
	): { x1: number; y1: number; x2: number; y2: number } {
		const dx = targetNode.x - sourceNode.x;
		const dy = targetNode.y - sourceNode.y;
		const dist = Math.sqrt(dx * dx + dy * dy);
		if (dist === 0)
			return { x1: sourceNode.x, y1: sourceNode.y, x2: targetNode.x, y2: targetNode.y };

		const nodeRadius = NODE_RADIUS + 2;
		const arrowOffset = 6;
		const ux = dx / dist;
		const uy = dy / dist;

		return {
			x1: sourceNode.x + ux * nodeRadius,
			y1: sourceNode.y + uy * nodeRadius,
			x2: targetNode.x - ux * (nodeRadius + arrowOffset),
			y2: targetNode.y - uy * (nodeRadius + arrowOffset)
		};
	}
</script>

<PageContainer breadcrumbs={[{ title: 'Traces', href: '/traces' }, { title: 'Service Map' }]}>
	<div class="flex flex-col gap-6">
		<!-- Header -->
		<div class="flex items-start justify-between">
			<div class="flex items-center gap-3">
				<div class="rounded-lg bg-primary/10 p-2">
					<NetworkIcon class="size-6 text-primary" />
				</div>
				<div>
					<h1 class="text-xl font-semibold">Service Map</h1>
					<p class="text-sm text-muted-foreground">
						Visualize service dependencies and communication patterns
					</p>
				</div>
			</div>
		</div>

		{#if serviceMapQuery.isPending}
			<div class="flex h-96 items-center justify-center text-muted-foreground">
				Loading service map...
			</div>
		{:else if graphData.nodes.length === 0}
			<div class="flex h-96 items-center justify-center rounded-lg border border-dashed">
				<div class="text-center">
					<NetworkIcon class="mx-auto mb-3 size-12 text-muted-foreground/50" />
					<h3 class="text-lg font-medium text-muted-foreground">No service connections</h3>
					<p class="mt-1 text-sm text-muted-foreground/70">
						No cross-service communication detected in the last 24 hours
					</p>
				</div>
			</div>
		{:else}
			<!-- Legend -->
			<div class="flex flex-wrap items-center gap-4 text-xs text-muted-foreground">
				<div class="flex items-center gap-1.5">
					<div class="h-0.5 w-4 rounded bg-emerald-500"></div>
					<span>Healthy</span>
				</div>
				<div class="flex items-center gap-1.5">
					<div class="h-0.5 w-4 rounded bg-amber-500"></div>
					<span>&lt;1% errors</span>
				</div>
				<div class="flex items-center gap-1.5">
					<div class="h-0.5 w-4 rounded bg-orange-500"></div>
					<span>1-5% errors</span>
				</div>
				<div class="flex items-center gap-1.5">
					<div class="h-0.5 w-4 rounded bg-red-500"></div>
					<span>&gt;5% errors</span>
				</div>
				<div class="ml-auto text-muted-foreground/60">
					Edge width = request volume · Click a node to inspect a service
				</div>
			</div>

			<!-- Graph -->
			<div class="relative overflow-hidden rounded-lg border bg-card">
				<!-- svelte-ignore a11y_no_static_element_interactions -->
				<svg
					viewBox={`0 0 ${GRAPH_WIDTH} ${GRAPH_HEIGHT}`}
					class="h-[680px] w-full"
					onmouseleave={handleMouseLeave}
				>
					<!-- Background grid pattern -->
					<defs>
						<pattern id="grid" width="40" height="40" patternUnits="userSpaceOnUse">
							<path
								d="M 40 0 L 0 0 0 40"
								fill="none"
								stroke="currentColor"
								stroke-width="0.3"
								class="text-border/30"
							/>
						</pattern>
						<marker id="arrowhead" markerWidth="8" markerHeight="6" refX="7" refY="3" orient="auto">
							<path d="M0,0 L8,3 L0,6 Z" fill="#a1a1aa" />
						</marker>
						{#each graphData.edges as edge, i}
							{@const color = getEdgeColor(edge.errorRate)}
							<marker
								id="arrow-{i}"
								markerWidth="8"
								markerHeight="6"
								refX="7"
								refY="3"
								orient="auto"
							>
								<path d="M0,0 L8,3 L0,6 Z" fill={color} />
							</marker>
						{/each}
					</defs>
					<rect width={GRAPH_WIDTH} height={GRAPH_HEIGHT} fill="url(#grid)" />

					<!-- Edges -->
					{#each graphData.edges as edge, i}
						{@const sourceNode = graphData.nodes.find((n) => n.id === edge.source)}
						{@const targetNode = graphData.nodes.find((n) => n.id === edge.target)}
						{#if sourceNode && targetNode}
							{@const path = computeEdgePath(sourceNode, targetNode)}
							{@const color = getEdgeColor(edge.errorRate)}
							{@const strokeW = getEdgeStrokeWidth(edge.requestCount, maxRequestCount)}

							<!-- Invisible wider hit area for hover -->
							<!-- svelte-ignore a11y_no_static_element_interactions -->
							<line
								x1={path.x1}
								y1={path.y1}
								x2={path.x2}
								y2={path.y2}
								stroke="transparent"
								stroke-width={Math.max(strokeW + 10, 16)}
								onmouseenter={(e) => handleEdgeMouseEnter(e, edge)}
								onmouseleave={handleMouseLeave}
								class="cursor-pointer"
							/>

							<!-- Visible edge -->
							<line
								x1={path.x1}
								y1={path.y1}
								x2={path.x2}
								y2={path.y2}
								stroke={color}
								stroke-width={strokeW}
								stroke-linecap="round"
								marker-end="url(#arrow-{i})"
								opacity={hoveredEdge
									? hoveredEdge === edge
										? 0.95
										: 0.18
									: selectedNodeId &&
										  edge.source !== selectedNodeId &&
										  edge.target !== selectedNodeId
										? 0.16
										: 0.82}
								class="pointer-events-none transition-opacity duration-150"
							/>

							<!-- Edge label (request count) -->
							{@const midX = (path.x1 + path.x2) / 2}
							{@const midY = (path.y1 + path.y2) / 2}
							<text
								x={midX}
								y={midY - 8}
								text-anchor="middle"
								class="pointer-events-none fill-muted-foreground text-[10px]"
								opacity={hoveredEdge
									? hoveredEdge === edge
										? 0.85
										: 0.2
									: selectedNodeId &&
										  edge.source !== selectedNodeId &&
										  edge.target !== selectedNodeId
										? 0.14
										: 0.68}
							>
								{formatCount(edge.requestCount)} req
							</text>
						{/if}
					{/each}

					<!-- Nodes -->
					{#each graphData.nodes as node (node.id)}
						{@const isHovered = hoveredNode?.id === node.id}
						{@const isSelected = selectedNodeId === node.id}
						{@const isActive = isHovered || isSelected}

						<g
							class="cursor-pointer"
							role="button"
							tabindex="0"
							aria-label={`Open ${node.id} service details`}
							aria-pressed={isSelected}
							onmouseenter={(e) => handleNodeMouseEnter(e, node)}
							onmouseleave={handleMouseLeave}
							onclick={() => handleNodeSelect(node.id)}
							onkeydown={(event) => handleNodeKeydown(event, node.id)}
						>
							<!-- Node glow on hover -->
							{#if isActive}
								<circle
									cx={node.x}
									cy={node.y}
									r={isSelected ? 42 : 38}
									fill={node.color}
									opacity={isSelected ? 0.18 : 0.12}
								/>
							{/if}

							<!-- Node circle -->
							<circle
								cx={node.x}
								cy={node.y}
								r={NODE_RADIUS}
								fill="var(--color-card)"
								stroke={node.color}
								stroke-width={isSelected ? 3 : isHovered ? 2.5 : 2}
								class="transition-all duration-150"
							/>

							<!-- Service initial -->
							<text
								x={node.x}
								y={node.y}
								text-anchor="middle"
								dominant-baseline="central"
								fill={node.color}
								class="text-sm font-bold"
								style="font-family: var(--font-mono, monospace);"
							>
								{node.id.charAt(0).toUpperCase()}
							</text>

							<!-- Service name label -->
							<text
								x={node.x}
								y={node.y + 48}
								text-anchor="middle"
								class="fill-foreground text-xs font-medium"
							>
								{node.id}
							</text>

							<!-- Error indicator -->
							{#if node.totalErrors > 0}
								<circle
									cx={node.x + 22}
									cy={node.y - 22}
									r="8"
									fill="#ef4444"
									stroke="var(--color-card)"
									stroke-width="2"
								/>
								<text
									x={node.x + 22}
									y={node.y - 22}
									text-anchor="middle"
									dominant-baseline="central"
									fill="white"
									class="text-[8px] font-bold"
								>
									!
								</text>
							{/if}
						</g>
					{/each}
				</svg>

				<!-- Tooltip -->
				{#if hoveredEdge}
					{@const errorRate = hoveredEdge.errorRate}
					<div
						class="pointer-events-none fixed z-50 rounded-lg border bg-popover px-3 py-2 text-sm shadow-lg"
						style="left: {tooltipX + 12}px; top: {tooltipY - 10}px;"
					>
						<div class="mb-1 font-medium">
							{hoveredEdge.source} &rarr; {hoveredEdge.target}
						</div>
						<div class="space-y-0.5 text-xs text-muted-foreground">
							<div class="flex justify-between gap-4">
								<span>Requests</span>
								<span class="font-mono">{formatCount(hoveredEdge.requestCount)}</span>
							</div>
							<div class="flex justify-between gap-4">
								<span>Errors</span>
								<span class="font-mono {hoveredEdge.errorCount > 0 ? 'text-red-500' : ''}">
									{formatCount(hoveredEdge.errorCount)}
								</span>
							</div>
							<div class="flex justify-between gap-4">
								<span>Error Rate</span>
								<span class="font-mono {errorRate > 0 ? 'text-red-500' : 'text-emerald-500'}">
									{errorRate.toFixed(2)}%
								</span>
							</div>
							<div class="flex justify-between gap-4">
								<span>Avg Latency</span>
								<span class="font-mono">{formatDuration(hoveredEdge.avgDurationNs)}</span>
							</div>
						</div>
					</div>
				{/if}

				{#if hoveredNode}
					<div
						class="pointer-events-none fixed z-50 rounded-lg border bg-popover px-3 py-2 text-sm shadow-lg"
						style="left: {tooltipX + 12}px; top: {tooltipY - 10}px;"
					>
						<div class="mb-1 font-medium">{hoveredNode.id}</div>
						<div class="space-y-0.5 text-xs text-muted-foreground">
							<div class="flex justify-between gap-4">
								<span>Total Requests</span>
								<span class="font-mono">{formatCount(hoveredNode.totalRequests)}</span>
							</div>
							<div class="flex justify-between gap-4">
								<span>Total Errors</span>
								<span class="font-mono {hoveredNode.totalErrors > 0 ? 'text-red-500' : ''}">
									{formatCount(hoveredNode.totalErrors)}
								</span>
							</div>
						</div>
					</div>
				{/if}
			</div>

			<Sheet.Root
				bind:open={
					() => selectedNodeId !== null,
					(open) => {
						if (!open) selectedNodeId = null;
					}
				}
			>
				<Sheet.Content side="right" class="w-full gap-0 overflow-hidden sm:max-w-[440px]">
					{#if selectedNodeDetails}
						<Sheet.Header class="border-b px-6 py-5">
							<div class="flex items-start gap-3">
								<div
									class="mt-1 size-3 rounded-full"
									style:background-color={selectedNodeDetails.node.color}
								></div>
								<div class="min-w-0">
									<Sheet.Title class="truncate pr-8">
										{selectedNodeDetails.node.id}
									</Sheet.Title>
									<Sheet.Description>
										Requests, errors, and direct dependencies in the current time window.
									</Sheet.Description>
								</div>
							</div>
						</Sheet.Header>

						<div class="flex-1 overflow-y-auto px-6 py-5">
							<div class="grid grid-cols-2 gap-3">
								<div class="rounded-lg border bg-muted/20 p-3">
									<div class="text-[11px] tracking-[0.14em] text-muted-foreground uppercase">
										Total Traffic
									</div>
									<div class="mt-1 text-lg font-semibold">
										{formatCount(selectedNodeDetails.node.totalRequests)}
									</div>
								</div>
								<div class="rounded-lg border bg-muted/20 p-3">
									<div class="text-[11px] tracking-[0.14em] text-muted-foreground uppercase">
										Errors
									</div>
									<div
										class={`mt-1 text-lg font-semibold ${selectedNodeDetails.node.totalErrors > 0 ? 'text-red-500' : ''}`}
									>
										{formatCount(selectedNodeDetails.node.totalErrors)}
									</div>
								</div>
								<div class="rounded-lg border bg-muted/20 p-3">
									<div class="text-[11px] tracking-[0.14em] text-muted-foreground uppercase">
										Error Rate
									</div>
									<div
										class={`mt-1 text-lg font-semibold ${selectedNodeDetails.errorRate > 0 ? 'text-red-500' : 'text-emerald-500'}`}
									>
										{selectedNodeDetails.errorRate.toFixed(2)}%
									</div>
								</div>
								<div class="rounded-lg border bg-muted/20 p-3">
									<div class="text-[11px] tracking-[0.14em] text-muted-foreground uppercase">
										Direct Peers
									</div>
									<div class="mt-1 text-lg font-semibold">
										{selectedNodeDetails.connectedServices}
									</div>
								</div>
							</div>

							<Separator class="my-5" />

							<div class="space-y-5">
								<section class="space-y-3">
									<div class="flex items-center justify-between gap-3">
										<div>
											<h3 class="text-sm font-semibold">Incoming calls</h3>
											<p class="text-xs text-muted-foreground">
												Requests arriving from upstream services
											</p>
										</div>
										<Badge variant="outline">
											{formatCount(selectedNodeDetails.incomingRequests)} req
										</Badge>
									</div>

									{#if selectedNodeDetails.incoming.length === 0}
										<div
											class="rounded-lg border border-dashed px-3 py-4 text-sm text-muted-foreground"
										>
											No upstream services in the current window.
										</div>
									{:else}
										<div class="space-y-2">
											{#each selectedNodeDetails.incoming as connection (connection.source + ':' + connection.target)}
												<button
													type="button"
													class="flex w-full items-start justify-between gap-3 rounded-lg border px-3 py-3 text-left transition-colors hover:border-primary/40 hover:bg-muted/30"
													onclick={() => handleNodeSelect(connection.counterpart)}
												>
													<div class="min-w-0">
														<div class="truncate font-medium">{connection.counterpart}</div>
														<div class="mt-1 text-xs text-muted-foreground">
															{formatCount(connection.requestCount)} requests · {formatDuration(
																connection.avgDurationNs
															)} avg · {connection.errorRate.toFixed(2)}% errors
														</div>
													</div>
													<Badge variant={connection.errorCount > 0 ? 'destructive' : 'outline'}>
														{formatCount(connection.errorCount)} err
													</Badge>
												</button>
											{/each}
										</div>
									{/if}
								</section>

								<section class="space-y-3">
									<div class="flex items-center justify-between gap-3">
										<div>
											<h3 class="text-sm font-semibold">Outgoing calls</h3>
											<p class="text-xs text-muted-foreground">
												Requests sent to downstream services
											</p>
										</div>
										<Badge variant="outline">
											{formatCount(selectedNodeDetails.outgoingRequests)} req
										</Badge>
									</div>

									{#if selectedNodeDetails.outgoing.length === 0}
										<div
											class="rounded-lg border border-dashed px-3 py-4 text-sm text-muted-foreground"
										>
											No downstream services in the current window.
										</div>
									{:else}
										<div class="space-y-2">
											{#each selectedNodeDetails.outgoing as connection (connection.source + ':' + connection.target)}
												<button
													type="button"
													class="flex w-full items-start justify-between gap-3 rounded-lg border px-3 py-3 text-left transition-colors hover:border-primary/40 hover:bg-muted/30"
													onclick={() => handleNodeSelect(connection.counterpart)}
												>
													<div class="min-w-0">
														<div class="truncate font-medium">{connection.counterpart}</div>
														<div class="mt-1 text-xs text-muted-foreground">
															{formatCount(connection.requestCount)} requests · {formatDuration(
																connection.avgDurationNs
															)} avg · {connection.errorRate.toFixed(2)}% errors
														</div>
													</div>
													<Badge variant={connection.errorCount > 0 ? 'destructive' : 'outline'}>
														{formatCount(connection.errorCount)} err
													</Badge>
												</button>
											{/each}
										</div>
									{/if}
								</section>
							</div>
						</div>
					{/if}
				</Sheet.Content>
			</Sheet.Root>
		{/if}
	</div>
</PageContainer>
