<script lang="ts">
	import PageContainer from '../../_components/page-container/page-container.svelte';
	import DatabaseIcon from '@lucide/svelte/icons/database';
</script>

<PageContainer breadcrumbs={[{ title: 'Traces', href: '/traces' }, { title: 'Sources' }]}>
	<div class="flex flex-col gap-6">
		<!-- Header -->
		<div class="flex items-start justify-between">
			<div class="flex items-center gap-3">
				<div class="rounded-lg bg-primary/10 p-2">
					<DatabaseIcon class="size-6 text-primary" />
				</div>
				<div>
					<h1 class="text-xl font-semibold">Trace Sources</h1>
					<p class="text-sm text-muted-foreground">
						Manage your trace ingestion sources and pipelines
					</p>
				</div>
			</div>
		</div>

		<!-- Placeholder -->
		<div class="flex h-96 items-center justify-center rounded-lg border border-dashed">
			<div class="text-center">
				<DatabaseIcon class="mx-auto mb-3 size-12 text-muted-foreground/50" />
				<h3 class="text-lg font-medium text-muted-foreground">Coming Soon</h3>
				<p class="mt-1 text-sm text-muted-foreground/70">
					Trace source management is under development
				</p>
			</div>
		</div>
	</div>
</PageContainer>
