<script lang="ts">
	import PageContainer from '../../_components/page-container/page-container.svelte';
	import NetworkIcon from '@lucide/svelte/icons/network';
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

		<!-- Placeholder -->
		<div class="flex h-96 items-center justify-center rounded-lg border border-dashed">
			<div class="text-center">
				<NetworkIcon class="mx-auto mb-3 size-12 text-muted-foreground/50" />
				<h3 class="text-lg font-medium text-muted-foreground">Coming Soon</h3>
				<p class="mt-1 text-sm text-muted-foreground/70">
					Service map visualization is under development
				</p>
			</div>
		</div>
	</div>
</PageContainer>
