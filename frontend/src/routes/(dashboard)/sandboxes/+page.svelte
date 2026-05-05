<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import {
		createCancelSandboxJob,
		createGetSandboxJob,
		createGetSandboxJobLogs,
		createListSandboxJobs
	} from '$lib/api/endpoints/sandbox/sandbox';
	import type { SandboxJob, SandboxJobLog } from '$lib/api/model';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import * as Card from '$lib/components/ui/card/index.js';
	import PageContainer from '../_components/page-container/page-container.svelte';
	import { sessionStore } from '$lib/stores/session';
	import BotIcon from '@lucide/svelte/icons/bot';
	import BoxesIcon from '@lucide/svelte/icons/boxes';
	import FileTextIcon from '@lucide/svelte/icons/file-text';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';

	type JobFilter = 'active' | 'all' | 'failed';

	let filter = $state<JobFilter>('active');

	const orgId = $derived($sessionStore?.active_organization?.id ?? '');

	const jobsQuery = createListSandboxJobs(
		() => orgId,
		() => ({ limit: 50 }),
		() => ({
			query: {
				enabled: !!orgId,
				refetchInterval: 5_000
			}
		})
	);

	const selectedJobId = $derived(page.url.searchParams.get('job') ?? '');

	const jobQuery = createGetSandboxJob(
		() => orgId,
		() => selectedJobId,
		() => ({
			query: {
				enabled: !!orgId && !!selectedJobId,
				refetchInterval: selectedJobId ? 4_000 : false
			}
		})
	);

	const logsQuery = createGetSandboxJobLogs(
		() => orgId,
		() => selectedJobId,
		() => ({ cursor: 0, limit: 400 }),
		() => ({
			query: {
				enabled: !!orgId && !!selectedJobId,
				refetchInterval: selectedJobId ? 4_000 : false
			}
		})
	);

	const cancelMutation = createCancelSandboxJob();

	const jobs = $derived.by((): SandboxJob[] => {
		const response = jobsQuery.data;
		if (response?.status === 200) {
			return response.data.jobs ?? [];
		}
		return [];
	});

	const filteredJobs = $derived.by((): SandboxJob[] => {
		switch (filter) {
			case 'active':
				return jobs.filter((job) => job.state === 'queued' || job.state === 'running');
			case 'failed':
				return jobs.filter((job) => job.state === 'failed' || job.state === 'timed_out');
			default:
				return jobs;
		}
	});

	const selectedJob = $derived.by((): SandboxJob | null => {
		const response = jobQuery.data;
		if (response?.status === 200) {
			return response.data;
		}
		return filteredJobs.find((job) => job.id === selectedJobId) ?? null;
	});

	const logs = $derived.by((): SandboxJobLog[] => {
		const response = logsQuery.data;
		if (response?.status === 200) {
			return response.data.logs ?? [];
		}
		return [];
	});

	const activeCount = $derived(jobs.filter((job) => job.state === 'queued' || job.state === 'running').length);
	const failedCount = $derived(
		jobs.filter((job) => job.state === 'failed' || job.state === 'timed_out').length
	);
	const draftPRCount = $derived(jobs.filter((job) => job.draft_pr).length);

	const averageRuntimeLabel = $derived.by(() => {
		const completed = jobs.filter((job) => job.started_at && job.finished_at);
		if (completed.length === 0) return 'n/a';

		const durations = completed.map((job) => {
			return new Date(job.finished_at ?? '').getTime() - new Date(job.started_at ?? '').getTime();
		});
		const average = durations.reduce((sum, value) => sum + value, 0) / durations.length;
		return formatDurationMs(average);
	});

	$effect(() => {
		if (!selectedJobId && filteredJobs.length > 0) {
			void selectJob(filteredJobs[0].id, true);
		}
	});

	async function selectJob(jobId: string, replaceState = false) {
		const params = new URLSearchParams(page.url.searchParams);
		params.set('job', jobId);
		await goto(`/sandboxes?${params.toString()}`, {
			replaceState,
			noScroll: true,
			keepFocus: true
		});
	}

	async function refreshAll() {
		await Promise.all([jobsQuery.refetch(), jobQuery.refetch(), logsQuery.refetch()]);
	}

	async function cancelSelectedJob() {
		if (!orgId || !selectedJob) return;
		await cancelMutation.mutateAsync({ organizationId: orgId, jobId: selectedJob.id });
		await refreshAll();
	}

	function stateVariant(
		state: string
	): 'default' | 'secondary' | 'destructive' | 'outline' {
		switch (state) {
			case 'running':
				return 'default';
			case 'queued':
				return 'secondary';
			case 'failed':
			case 'timed_out':
				return 'destructive';
			default:
				return 'outline';
		}
	}

	function streamClass(stream: string): string {
		switch (stream) {
			case 'stderr':
				return 'text-rose-300';
			case 'stdout':
				return 'text-emerald-300';
			default:
				return 'text-sky-300';
		}
	}

	function formatDate(value?: string): string {
		if (!value) return 'n/a';
		return new Date(value).toLocaleString();
	}

	function relativeTime(value?: string): string {
		if (!value) return 'n/a';
		const diffMs = Date.now() - new Date(value).getTime();
		const diffSeconds = Math.max(0, Math.floor(diffMs / 1000));
		if (diffSeconds < 60) return `${diffSeconds}s ago`;
		if (diffSeconds < 3600) return `${Math.floor(diffSeconds / 60)}m ago`;
		return `${Math.floor(diffSeconds / 3600)}h ago`;
	}

	function formatDuration(job: SandboxJob | null): string {
		if (!job?.started_at) return 'waiting to start';
		const end = job.finished_at ? new Date(job.finished_at).getTime() : Date.now();
		const start = new Date(job.started_at).getTime();
		return formatDurationMs(end - start);
	}

	function formatDurationMs(value: number): string {
		const totalSeconds = Math.max(0, Math.round(value / 1000));
		const minutes = Math.floor(totalSeconds / 60);
		const seconds = totalSeconds % 60;
		if (minutes === 0) return `${seconds}s`;
		return `${minutes}m ${seconds}s`;
	}
</script>

<PageContainer breadcrumbs={[{ title: 'Automation', href: '/metrics/services' }, { title: 'Sandboxes' }]}>
	<div class="flex flex-col gap-4">
		<div class="flex flex-wrap items-start justify-between gap-3">
			<div class="flex items-center gap-3">
				<div class="rounded-2xl border border-primary/20 bg-primary/10 p-3">
					<BoxesIcon class="size-6 text-primary" />
				</div>
				<div class="space-y-1">
					<h1 class="text-xl font-semibold">Sandbox Runs</h1>
					<p class="text-sm text-muted-foreground">
						Watch queued and running auto-resolve sandboxes, inspect their logs, and jump into the job that is currently shaping a pull request.
					</p>
				</div>
			</div>

			<div class="flex flex-wrap gap-2">
				<Button variant={filter === 'active' ? 'default' : 'outline'} onclick={() => (filter = 'active')}>
					Active
				</Button>
				<Button variant={filter === 'all' ? 'default' : 'outline'} onclick={() => (filter = 'all')}>
					All
				</Button>
				<Button variant={filter === 'failed' ? 'default' : 'outline'} onclick={() => (filter = 'failed')}>
					Failed
				</Button>
				<Button variant="outline" onclick={refreshAll}>
					<RefreshCwIcon class="mr-2 size-4" />
					Refresh
				</Button>
			</div>
		</div>

		<div class="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
			<Card.Root class="border-primary/20 bg-linear-to-br from-primary/10 via-background to-background">
				<Card.Content class="pt-6">
					<div class="text-xs uppercase tracking-[0.24em] text-muted-foreground">Active</div>
					<div class="mt-2 text-3xl font-semibold">{activeCount}</div>
					<div class="mt-1 text-sm text-muted-foreground">Queued or currently running sandboxes.</div>
				</Card.Content>
			</Card.Root>
			<Card.Root>
				<Card.Content class="pt-6">
					<div class="text-xs uppercase tracking-[0.24em] text-muted-foreground">Failed</div>
					<div class="mt-2 text-3xl font-semibold">{failedCount}</div>
					<div class="mt-1 text-sm text-muted-foreground">Runs that need investigation or a retry.</div>
				</Card.Content>
			</Card.Root>
			<Card.Root>
				<Card.Content class="pt-6">
					<div class="text-xs uppercase tracking-[0.24em] text-muted-foreground">Draft PRs</div>
					<div class="mt-2 text-3xl font-semibold">{draftPRCount}</div>
					<div class="mt-1 text-sm text-muted-foreground">Jobs currently configured to open draft pull requests.</div>
				</Card.Content>
			</Card.Root>
			<Card.Root>
				<Card.Content class="pt-6">
					<div class="text-xs uppercase tracking-[0.24em] text-muted-foreground">Average Runtime</div>
					<div class="mt-2 text-3xl font-semibold">{averageRuntimeLabel}</div>
					<div class="mt-1 text-sm text-muted-foreground">Across completed sandbox runs in the current view.</div>
				</Card.Content>
			</Card.Root>
		</div>

		<div class="grid gap-4 xl:grid-cols-[360px,1fr]">
			<Card.Root class="overflow-hidden">
				<Card.Header class="border-b bg-muted/20">
					<div class="flex items-center justify-between gap-3">
						<div>
							<Card.Title class="text-base">Job Feed</Card.Title>
							<Card.Description>Click any run to inspect state, timeline, and sandbox logs.</Card.Description>
						</div>
						<Badge variant="outline">{filteredJobs.length} visible</Badge>
					</div>
				</Card.Header>
				<Card.Content class="p-0">
					<div class="max-h-[70vh] overflow-auto">
						{#if jobsQuery.isPending}
							<div class="p-6 text-sm text-muted-foreground">Loading sandbox jobs...</div>
						{:else if filteredJobs.length === 0}
							<div class="p-6 text-sm text-muted-foreground">No sandbox jobs match this view yet.</div>
						{:else}
							<div class="divide-y">
								{#each filteredJobs as job (job.id)}
									<button
										type="button"
										class={`flex w-full flex-col gap-3 px-4 py-4 text-left transition-colors ${
											selectedJobId === job.id ? 'bg-primary/8' : 'hover:bg-muted/30'
										}`}
										onclick={() => selectJob(job.id)}
									>
										<div class="flex items-start justify-between gap-3">
											<div class="space-y-1">
												<div class="font-medium">{job.task_title || job.commit_message}</div>
												<div class="text-xs text-muted-foreground">{job.id} • {job.mode.replace('_', ' ')}</div>
											</div>
											<Badge variant={stateVariant(job.state)}>{job.state}</Badge>
										</div>
										<div class="grid gap-2 text-xs text-muted-foreground sm:grid-cols-2">
											<div>Repo: {job.repository_id}</div>
											<div>Age: {relativeTime(job.created_at)}</div>
											<div>Branch: {job.branch_name || 'pending'}</div>
											<div>Duration: {formatDuration(job)}</div>
										</div>
									</button>
								{/each}
							</div>
						{/if}
					</div>
				</Card.Content>
			</Card.Root>

			{#if selectedJob}
				<div class="flex flex-col gap-4">
					<Card.Root class="overflow-hidden border-primary/20 bg-linear-to-br from-background via-background to-primary/5">
						<Card.Header class="gap-3 border-b bg-muted/10">
							<div class="flex flex-wrap items-start justify-between gap-3">
								<div class="space-y-1">
									<div class="flex items-center gap-2">
										<Card.Title class="text-lg">{selectedJob.task_title || selectedJob.commit_message}</Card.Title>
										<Badge variant={stateVariant(selectedJob.state)}>{selectedJob.state}</Badge>
									</div>
									<Card.Description>
										{selectedJob.id} • {selectedJob.mode.replace('_', ' ')} • created {relativeTime(selectedJob.created_at)}
									</Card.Description>
								</div>

								<div class="flex flex-wrap gap-2">
									{#if (selectedJob.state === 'queued' || selectedJob.state === 'running') && !selectedJob.cancel_requested}
										<Button
											variant="outline"
											onclick={cancelSelectedJob}
											disabled={cancelMutation.isPending}
										>
											{cancelMutation.isPending ? 'Cancelling...' : 'Cancel Run'}
										</Button>
									{/if}
									{#if selectedJob.pull_request_url}
										<Button href={selectedJob.pull_request_url} target="_blank">Open PR</Button>
									{/if}
								</div>
							</div>
						</Card.Header>
						<Card.Content class="grid gap-4 pt-6 md:grid-cols-3">
							<div class="rounded-xl border bg-background/80 p-4">
								<div class="flex items-center gap-2 text-sm font-medium">
									<BotIcon class="size-4 text-primary" />
									Execution
								</div>
								<div class="mt-3 space-y-2 text-sm">
									<div class="flex justify-between gap-3"><span class="text-muted-foreground">Runtime</span><span>{selectedJob.runtime_type || 'pending'}</span></div>
									<div class="flex justify-between gap-3"><span class="text-muted-foreground">Sandbox</span><span class="font-mono text-xs">{selectedJob.sandbox_instance_id || 'pending'}</span></div>
									<div class="flex justify-between gap-3"><span class="text-muted-foreground">Duration</span><span>{formatDuration(selectedJob)}</span></div>
									<div class="flex justify-between gap-3"><span class="text-muted-foreground">Cancel requested</span><span>{selectedJob.cancel_requested ? 'yes' : 'no'}</span></div>
								</div>
							</div>

							<div class="rounded-xl border bg-background/80 p-4">
								<div class="flex items-center gap-2 text-sm font-medium">
									<FileTextIcon class="size-4 text-primary" />
									Git
								</div>
								<div class="mt-3 space-y-2 text-sm">
									<div class="flex justify-between gap-3"><span class="text-muted-foreground">Base branch</span><span>{selectedJob.base_branch}</span></div>
									<div class="flex justify-between gap-3"><span class="text-muted-foreground">Working branch</span><span>{selectedJob.branch_name || 'pending'}</span></div>
									<div class="flex justify-between gap-3"><span class="text-muted-foreground">Draft PR</span><span>{selectedJob.draft_pr ? 'yes' : 'no'}</span></div>
									<div class="flex justify-between gap-3"><span class="text-muted-foreground">PR number</span><span>{selectedJob.pull_request_number ?? 'pending'}</span></div>
								</div>
							</div>

							<div class="rounded-xl border bg-background/80 p-4">
								<div class="text-sm font-medium">Timeline</div>
								<div class="mt-3 space-y-2 text-sm">
									<div class="flex justify-between gap-3"><span class="text-muted-foreground">Created</span><span>{formatDate(selectedJob.created_at)}</span></div>
									<div class="flex justify-between gap-3"><span class="text-muted-foreground">Started</span><span>{formatDate(selectedJob.started_at)}</span></div>
									<div class="flex justify-between gap-3"><span class="text-muted-foreground">Finished</span><span>{formatDate(selectedJob.finished_at)}</span></div>
									<div class="flex justify-between gap-3"><span class="text-muted-foreground">Updated</span><span>{formatDate(selectedJob.updated_at)}</span></div>
								</div>
							</div>

							{#if selectedJob.error}
								<div class="md:col-span-3 rounded-xl border border-rose-300/60 bg-rose-500/10 p-4 text-sm text-rose-100">
									<div class="font-medium text-rose-200">Latest failure</div>
									<div class="mt-2 font-mono text-xs leading-6">{selectedJob.error}</div>
								</div>
							{/if}
						</Card.Content>
					</Card.Root>

					<Card.Root class="overflow-hidden">
						<Card.Header class="border-b bg-muted/20">
							<div class="flex items-center justify-between gap-3">
								<div>
									<Card.Title class="text-base">Live Logs</Card.Title>
									<Card.Description>
										Sandbox, command, and PR creation output for the selected run.
									</Card.Description>
								</div>
								<Badge variant="outline">{logs.length} lines</Badge>
							</div>
						</Card.Header>
						<Card.Content class="p-0">
							<div class="max-h-[48vh] overflow-auto bg-slate-950 px-4 py-4 font-mono text-xs leading-6 text-slate-100">
								{#if logsQuery.isPending}
									<div class="text-slate-400">Loading logs...</div>
								{:else if logs.length === 0}
									<div class="text-slate-400">No logs yet. Running sandboxes stream here as commands execute.</div>
								{:else}
									{#each logs as log (log.seq)}
										<div class="border-b border-slate-800/80 py-2 last:border-b-0">
											<div class="flex flex-wrap items-center gap-2 text-[10px] uppercase tracking-[0.18em] text-slate-500">
												<span>#{log.seq}</span>
												<span class={streamClass(log.stream)}>{log.stream}</span>
												<span>{new Date(log.created_at).toLocaleTimeString()}</span>
											</div>
											<div class="mt-1 whitespace-pre-wrap break-words text-slate-100">{log.message}</div>
										</div>
									{/each}
								{/if}
							</div>
						</Card.Content>
					</Card.Root>
				</div>
			{:else}
				<Card.Root class="flex min-h-[420px] items-center justify-center border-dashed">
					<Card.Content class="text-center">
						<div class="mx-auto mb-4 flex size-12 items-center justify-center rounded-full bg-primary/10">
							<BoxesIcon class="size-5 text-primary" />
						</div>
						<div class="text-base font-medium">Select a sandbox run</div>
						<p class="mt-2 text-sm text-muted-foreground">
							Pick a job from the feed to inspect its live logs, PR state, sandbox instance, and timeline.
						</p>
					</Card.Content>
				</Card.Root>
			{/if}
		</div>
	</div>
</PageContainer>
