<script lang="ts">
	import PageContainer from '../../_components/page-container/page-container.svelte';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import * as Table from '$lib/components/ui/table/index.js';
	import DatabaseIcon from '@lucide/svelte/icons/database';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import SettingsIcon from '@lucide/svelte/icons/settings';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import { logSources } from '../_components/mock-data';

	function getStatusBadgeVariant(
		status: string
	): 'default' | 'secondary' | 'destructive' | 'outline' {
		switch (status) {
			case 'active':
				return 'default';
			case 'inactive':
				return 'secondary';
			case 'error':
				return 'destructive';
			default:
				return 'outline';
		}
	}

	function formatLogsPerMinute(lpm: number): string {
		if (lpm === 0) return '-';
		if (lpm >= 1000) return `${(lpm / 1000).toFixed(1)}k/min`;
		return `${lpm}/min`;
	}
</script>

<PageContainer breadcrumbs={[{ title: 'Logs', href: '/logs' }, { title: 'Sources' }]}>
	<div class="flex flex-col gap-6">
		<!-- Header -->
		<div class="flex items-start justify-between">
			<div class="flex items-center gap-3">
				<div class="rounded-lg bg-primary/10 p-2">
					<DatabaseIcon class="size-6 text-primary" />
				</div>
				<div>
					<h1 class="text-xl font-semibold">Log Sources</h1>
					<p class="text-sm text-muted-foreground">
						Manage your log ingestion sources and pipelines
					</p>
				</div>
			</div>
			<Button>
				<PlusIcon class="mr-2 size-4" />
				Add Source
			</Button>
		</div>

		<!-- Sources Table -->
		<div class="rounded-lg border">
			<Table.Root>
				<Table.Header>
					<Table.Row>
						<Table.Head class="w-[250px]">Name</Table.Head>
						<Table.Head>Type</Table.Head>
						<Table.Head>Status</Table.Head>
						<Table.Head class="text-right">Throughput</Table.Head>
						<Table.Head class="w-[100px] text-right">Actions</Table.Head>
					</Table.Row>
				</Table.Header>
				<Table.Body>
					{#each logSources as source (source.id)}
						<Table.Row>
							<Table.Cell class="font-medium">{source.name}</Table.Cell>
							<Table.Cell>
								<Badge variant="outline" class="font-mono text-xs">
									{source.type}
								</Badge>
							</Table.Cell>
							<Table.Cell>
								<Badge variant={getStatusBadgeVariant(source.status)} class="capitalize">
									{source.status}
								</Badge>
							</Table.Cell>
							<Table.Cell class="text-right font-mono">
								{formatLogsPerMinute(source.logsPerMinute)}
							</Table.Cell>
							<Table.Cell class="text-right">
								<div class="flex justify-end gap-1">
									<Button variant="ghost" size="icon-sm">
										<RefreshCwIcon class="size-4" />
									</Button>
									<Button variant="ghost" size="icon-sm">
										<SettingsIcon class="size-4" />
									</Button>
								</div>
							</Table.Cell>
						</Table.Row>
					{/each}
				</Table.Body>
			</Table.Root>
		</div>

		<!-- Stats -->
		<div class="grid gap-4 md:grid-cols-3">
			<div class="rounded-lg border p-4">
				<p class="text-sm text-muted-foreground">Total Sources</p>
				<p class="text-2xl font-semibold">{logSources.length}</p>
			</div>
			<div class="rounded-lg border p-4">
				<p class="text-sm text-muted-foreground">Active Sources</p>
				<p class="text-2xl font-semibold">
					{logSources.filter((s) => s.status === 'active').length}
				</p>
			</div>
			<div class="rounded-lg border p-4">
				<p class="text-sm text-muted-foreground">Total Throughput</p>
				<p class="font-mono text-2xl font-semibold">
					{formatLogsPerMinute(logSources.reduce((sum, s) => sum + s.logsPerMinute, 0))}
				</p>
			</div>
		</div>
	</div>
</PageContainer>
