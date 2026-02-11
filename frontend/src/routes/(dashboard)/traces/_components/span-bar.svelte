<script lang="ts">
	let {
		offsetPercent = 0,
		widthPercent = 1,
		color = 'bg-primary',
		isError = false,
		isSelected = false,
		label = ''
	}: {
		offsetPercent?: number;
		widthPercent?: number;
		color?: string;
		isError?: boolean;
		isSelected?: boolean;
		label?: string;
	} = $props();

	// Ensure minimum visible width
	const displayWidth = $derived(Math.max(0.3, widthPercent));
</script>

<div class="group relative h-6 w-full">
	<!-- Background track -->
	<div class="absolute inset-0 rounded bg-muted/30"></div>

	<!-- Timing bar -->
	<div
		class="absolute top-0.5 bottom-0.5 rounded-sm transition-all {isError
			? 'bg-red-500'
			: color} {isSelected ? 'ring-2 ring-primary ring-offset-1 ring-offset-background' : ''}"
		style="left: {offsetPercent}%; width: {displayWidth}%;"
	>
		<!-- Duration label on hover -->
		{#if label}
			<div
				class="pointer-events-none absolute top-1/2 left-full ml-2 -translate-y-1/2 whitespace-nowrap rounded bg-popover px-1.5 py-0.5 text-xs font-medium text-popover-foreground opacity-0 shadow-sm transition-opacity group-hover:opacity-100"
			>
				{label}
			</div>
		{/if}
	</div>
</div>
