<script lang="ts">
	import PageContainer from '../../_components/page-container/page-container.svelte';
	import { Button } from '$lib/components/ui/button/index.js';
	import * as Card from '$lib/components/ui/card/index.js';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import BookmarkIcon from '@lucide/svelte/icons/bookmark';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import TrashIcon from '@lucide/svelte/icons/trash-2';
	import EditIcon from '@lucide/svelte/icons/pencil';
	import { savedViews } from '../_components/mock-data';
</script>

<PageContainer breadcrumbs={[{ title: 'Logs', href: '/logs' }, { title: 'Saved Views' }]}>
	<div class="flex flex-col gap-6">
		<!-- Header -->
		<div class="flex items-start justify-between">
			<div class="flex items-center gap-3">
				<div class="rounded-lg bg-primary/10 p-2">
					<BookmarkIcon class="size-6 text-primary" />
				</div>
				<div>
					<h1 class="text-xl font-semibold">Saved Views</h1>
					<p class="text-sm text-muted-foreground">
						Quick access to your frequently used log queries
					</p>
				</div>
			</div>
			<Button>
				<PlusIcon class="mr-2 size-4" />
				Create View
			</Button>
		</div>

		<!-- Views Grid -->
		<div class="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
			{#each savedViews as view (view.id)}
				<Card.Root class="cursor-pointer transition-colors hover:border-primary/50">
					<Card.Header>
						<div class="flex items-start justify-between">
							<Card.Title class="text-base">{view.name}</Card.Title>
							<div class="flex items-center gap-1">
								<Button variant="ghost" size="icon-sm">
									<EditIcon class="size-4" />
								</Button>
								<Button
									variant="ghost"
									size="icon-sm"
									class="text-destructive hover:text-destructive"
								>
									<TrashIcon class="size-4" />
								</Button>
							</div>
						</div>
						<Card.Description class="font-mono text-xs">
							{view.query}
						</Card.Description>
					</Card.Header>
					<Card.Content>
						<div class="flex flex-wrap gap-1">
							{#if view.filters.levels}
								{#each view.filters.levels as level (level)}
									<Badge variant="secondary" class="text-xs capitalize">{level}</Badge>
								{/each}
							{/if}
							{#if view.filters.services}
								{#each view.filters.services as service (service)}
									<Badge variant="outline" class="text-xs">{service}</Badge>
								{/each}
							{/if}
							{#if view.filters.hosts}
								<Badge variant="outline" class="text-xs">
									{view.filters.hosts.length} hosts
								</Badge>
							{/if}
						</div>
					</Card.Content>
					<Card.Footer>
						<Button variant="outline" size="sm" class="w-full" href="/logs?view={view.id}">
							Apply View
						</Button>
					</Card.Footer>
				</Card.Root>
			{/each}

			<!-- Empty state card -->
			<Card.Root class="flex min-h-[200px] items-center justify-center border-dashed">
				<div class="text-center text-muted-foreground">
					<PlusIcon class="mx-auto mb-2 size-8 opacity-50" />
					<p class="text-sm">Create a new view</p>
				</div>
			</Card.Root>
		</div>
	</div>
</PageContainer>
