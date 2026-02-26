<script lang="ts">
	import { page } from '$app/state';
	import PageContainer from '../_components/page-container/page-container.svelte';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import * as Card from '$lib/components/ui/card/index.js';
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import * as Select from '$lib/components/ui/select/index.js';
	import { Separator } from '$lib/components/ui/separator/index.js';
	import SettingsIcon from '@lucide/svelte/icons/settings';
	import KeyIcon from '@lucide/svelte/icons/key-round';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import TrashIcon from '@lucide/svelte/icons/trash-2';
	import CopyIcon from '@lucide/svelte/icons/copy';
	import CheckIcon from '@lucide/svelte/icons/check';
	import GithubIcon from '@lucide/svelte/icons/github';
	import GitBranchIcon from '@lucide/svelte/icons/git-branch';
	import PlayIcon from '@lucide/svelte/icons/play';
	import LoaderCircleIcon from '@lucide/svelte/icons/loader-circle';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import {
		createListApiKeys,
		createCreateApiKey,
		createRevokeApiKey,
		getListApiKeysQueryKey
	} from '$lib/api/endpoints/api-keys/api-keys';
	import {
		createCreateGithubInstallUrl,
		createListGithubInstallations,
		createListGithubRepositories,
		createSetGithubRepositoryEnabled,
		getListGithubInstallationsQueryKey,
		getListGithubRepositoriesQueryKey
	} from '$lib/api/endpoints/github/github';
	import {
		createSandboxJob,
		listSandboxJobs,
		getSandboxJob,
		getSandboxJobLogs,
		cancelSandboxJob
	} from '$lib/api/endpoints/sandbox/sandbox';
	import { createGetLogServices } from '$lib/api/endpoints/logs/logs';
	import {
		listServiceRemediationMappings,
		upsertServiceRemediationMapping,
		deleteServiceRemediationMapping,
		RemediationAPIError,
		type ServiceRemediationMapping
	} from '$lib/remediation/api';
	import { sessionStore } from '$lib/stores/session';
	import { useQueryClient } from '@tanstack/svelte-query';
	import type {
		ApiKey,
		GithubInstallation,
		GithubRepository,
		SandboxJob,
		SandboxJobLog
	} from '$lib/api/model';

	const orgId = $derived($sessionStore?.active_organization?.id ?? '');
	const orgName = $derived($sessionStore?.active_organization?.name ?? '');
	const queryClient = useQueryClient();
	const githubConnectState = $derived(page.url.searchParams.get('github'));
	const githubConnectCode = $derived(page.url.searchParams.get('code'));

	// List API keys
	const apiKeysQuery = createListApiKeys(() => orgId);

	const apiKeys = $derived.by((): ApiKey[] => {
		const resp = apiKeysQuery.data;
		if (resp && resp.status === 200) {
			return (resp.data.api_keys ?? []).filter((k) => !k.revoked_at);
		}
		return [];
	});

	// Create API key mutation
	const createKeyMutation = createCreateApiKey(() => ({
		mutation: {
			onSuccess: () => {
				queryClient.invalidateQueries({ queryKey: getListApiKeysQueryKey(orgId) });
			}
		}
	}));

	// Revoke API key mutation
	const revokeKeyMutation = createRevokeApiKey(() => ({
		mutation: {
			onSuccess: () => {
				queryClient.invalidateQueries({ queryKey: getListApiKeysQueryKey(orgId) });
			}
		}
	}));

	// GitHub queries/mutations
	const githubInstallationsQuery = createListGithubInstallations(() => orgId);
	const githubRepositoriesQuery = createListGithubRepositories(() => orgId);
	const logServicesQuery = createGetLogServices(() => orgId);
	const connectGithubMutation = createCreateGithubInstallUrl();
	const setGithubRepoEnabledMutation = createSetGithubRepositoryEnabled(() => ({
		mutation: {
			onSuccess: () => {
				queryClient.invalidateQueries({ queryKey: getListGithubInstallationsQueryKey(orgId) });
				queryClient.invalidateQueries({ queryKey: getListGithubRepositoriesQueryKey(orgId) });
			}
		}
	}));

	// Create key dialog state
	let createDialogOpen = $state(false);
	let newKeyName = $state('');
	let newKeyExpiry = $state<string>('never');
	let createdKeyValue = $state<string | null>(null);
	let copiedKey = $state(false);

	// Revoke confirmation
	let revokeDialogOpen = $state(false);
	let keyToRevoke = $state<ApiKey | null>(null);

	// Sandbox state
	let sandboxRepositoryID = $state('');
	let sandboxTaskTitle = $state('Automated fix');
	let sandboxCommitMessage = $state('chore: apply automated fix');
	let sandboxCommands = $state('go test ./...');
	let sandboxJob = $state<SandboxJob | null>(null);
	let sandboxLogs = $state<SandboxJobLog[]>([]);
	let sandboxLogsCursor = $state(0);
	let sandboxError = $state<string | null>(null);
	let sandboxSubmitting = $state(false);
	let sandboxPolling = $state(false);
	let runningSandboxJobs = $state<SandboxJob[]>([]);
	let runningJobsLoading = $state(false);
	let runningJobsManualRefresh = $state(false);
	let runningJobsError = $state<string | null>(null);
	let githubSyncing = $state(false);
	let remediationMappings = $state<ServiceRemediationMapping[]>([]);
	let remediationLoading = $state(false);
	let remediationSavingService = $state<string | undefined>(undefined);
	let remediationDeletingService = $state<string | undefined>(undefined);
	let remediationError = $state<string | null>(null);
	let remediationRepoByService = $state<Record<string, string>>({});

	const expiryOptions = [
		{ value: 'never', label: 'Never' },
		{ value: '30', label: '30 days' },
		{ value: '60', label: '60 days' },
		{ value: '90', label: '90 days' },
		{ value: '365', label: '1 year' }
	];

	const githubInstallations = $derived.by((): GithubInstallation[] => {
		const res = githubInstallationsQuery.data;
		if (res?.status === 200) {
			return res.data.installations ?? [];
		}
		return [];
	});

	const githubRepositories = $derived.by((): GithubRepository[] => {
		const res = githubRepositoriesQuery.data;
		if (res?.status === 200) {
			return res.data.repositories ?? [];
		}
		return [];
	});

	const enabledGithubRepositories = $derived.by((): GithubRepository[] => {
		return githubRepositories.filter((repo) => repo.enabled && !repo.archived);
	});

	const repoNameByID = $derived.by((): Record<string, string> => {
		const out: Record<string, string> = {};
		for (const repo of githubRepositories) {
			out[repo.id] = repo.full_name;
		}
		return out;
	});

	const logServices = $derived.by((): string[] => {
		const resp = logServicesQuery.data;
		if (resp?.status === 200) {
			return resp.data.services ?? [];
		}
		return [];
	});

	const mappedServices = $derived.by((): string[] => {
		return remediationMappings.map((mapping) => mapping.service_name);
	});

	const remediationServices = $derived.by((): string[] => {
		const union = new Set<string>([...logServices, ...mappedServices]);
		return [...union].sort((a, b) => a.localeCompare(b));
	});

	$effect(() => {
		if (enabledGithubRepositories.length === 0) {
			sandboxRepositoryID = '';
			return;
		}

		const selectedStillExists = enabledGithubRepositories.some((repo) => repo.id === sandboxRepositoryID);
		if (!selectedStillExists) {
			sandboxRepositoryID = enabledGithubRepositories[0].id;
		}
	});

	$effect(() => {
		orgId;
		void refreshRemediationMappings();
	});

	$effect(() => {
		const next: Record<string, string> = {};
		const firstEnabledRepoID = enabledGithubRepositories[0]?.id ?? '';

		for (const service of remediationServices) {
			const existing = remediationMappings.find((mapping) => mapping.service_name === service);
			next[service] = existing?.repository_id ?? firstEnabledRepoID;
		}

		remediationRepoByService = next;
	});

	$effect(() => {
		if (!orgId) {
			runningSandboxJobs = [];
			runningJobsError = null;
			return;
		}

		let cancelled = false;
		const pollIntervalMS = 5000;

		const poll = async () => {
			if (cancelled) return;
			await refreshRunningSandboxJobs();
		};

		void poll();
		const intervalID = setInterval(() => {
			void poll();
		}, pollIntervalMS);

		return () => {
			cancelled = true;
			clearInterval(intervalID);
		};
	});

	async function refreshRemediationMappings() {
		if (!orgId) {
			remediationMappings = [];
			remediationError = null;
			return;
		}
		remediationLoading = true;
		remediationError = null;
		try {
			remediationMappings = await listServiceRemediationMappings(orgId);
		} catch (error) {
			if (error instanceof RemediationAPIError) {
				remediationError = error.code;
			} else {
				remediationError = error instanceof Error ? error.message : 'failed to load mappings';
			}
		} finally {
			remediationLoading = false;
		}
	}

	async function handleCreateKey() {
		if (!newKeyName.trim()) return;
		const expiresIn = newKeyExpiry === 'never' ? undefined : parseInt(newKeyExpiry);
		createKeyMutation.mutate(
			{
				organizationId: orgId,
				data: { name: newKeyName.trim(), expires_in: expiresIn }
			},
			{
				onSuccess: (resp) => {
					if (resp.status === 200) {
						createdKeyValue = resp.data.key;
					}
				}
			}
		);
	}

	function closeCreateDialog() {
		createDialogOpen = false;
		newKeyName = '';
		newKeyExpiry = 'never';
		createdKeyValue = null;
		copiedKey = false;
	}

	function copyKey() {
		if (createdKeyValue) {
			navigator.clipboard.writeText(createdKeyValue);
			copiedKey = true;
			setTimeout(() => (copiedKey = false), 2000);
		}
	}

	function confirmRevoke(key: ApiKey) {
		keyToRevoke = key;
		revokeDialogOpen = true;
	}

	function handleRevoke() {
		if (!keyToRevoke) return;
		revokeKeyMutation.mutate(
			{ organizationId: orgId, keyId: keyToRevoke.id },
			{
				onSuccess: () => {
					revokeDialogOpen = false;
					keyToRevoke = null;
				}
			}
		);
	}

	function formatDate(iso: string): string {
		return new Date(iso).toLocaleDateString('en-US', {
			month: 'short',
			day: 'numeric',
			year: 'numeric'
		});
	}

	function formatRelative(iso: string | null): string {
		if (!iso) return 'Never';
		const date = new Date(iso);
		const now = new Date();
		const diff = now.getTime() - date.getTime();
		const minutes = Math.floor(diff / 60000);
		if (minutes < 1) return 'Just now';
		if (minutes < 60) return `${minutes}m ago`;
		const hours = Math.floor(minutes / 60);
		if (hours < 24) return `${hours}h ago`;
		const days = Math.floor(hours / 24);
		return `${days}d ago`;
	}

	function connectGithub() {
		if (!orgId || connectGithubMutation.isPending) return;
		connectGithubMutation.mutate(
			{
				organizationId: orgId
			},
			{
				onSuccess: (res) => {
					if (res.status === 200 && res.data.url) {
						window.location.href = res.data.url;
					}
				}
			}
		);
	}

	async function syncGithubRepositories() {
		if (!orgId || githubSyncing) return;
		githubSyncing = true;
		try {
			await queryClient.invalidateQueries({ queryKey: getListGithubInstallationsQueryKey(orgId) });
			await queryClient.invalidateQueries({ queryKey: getListGithubRepositoriesQueryKey(orgId) });
		} finally {
			githubSyncing = false;
		}
	}

	function toggleGithubRepositoryEnabled(repo: GithubRepository) {
		setGithubRepoEnabledMutation.mutate({
			organizationId: orgId,
			repositoryId: repo.id,
			data: {
				enabled: !repo.enabled
			}
		});
	}

	async function saveRemediationMapping(serviceName: string) {
		if (!orgId || remediationSavingService || !serviceName) return;
		const repositoryID = remediationRepoByService[serviceName];
		if (!repositoryID) {
			remediationError = 'select a repository before saving';
			return;
		}

		remediationSavingService = serviceName;
		remediationError = null;
		try {
			await upsertServiceRemediationMapping(orgId, serviceName, repositoryID);
			await refreshRemediationMappings();
		} catch (error) {
			if (error instanceof RemediationAPIError) {
				remediationError = error.code;
			} else {
				remediationError = error instanceof Error ? error.message : 'failed to save mapping';
			}
		} finally {
			remediationSavingService = undefined;
		}
	}

	async function removeRemediationMapping(serviceName: string) {
		if (!orgId || remediationDeletingService || !serviceName) return;

		remediationDeletingService = serviceName;
		remediationError = null;
		try {
			await deleteServiceRemediationMapping(orgId, serviceName);
			await refreshRemediationMappings();
		} catch (error) {
			if (error instanceof RemediationAPIError) {
				remediationError = error.code;
			} else {
				remediationError = error instanceof Error ? error.message : 'failed to delete mapping';
			}
		} finally {
			remediationDeletingService = undefined;
		}
	}

	function formatSandboxState(state: string): string {
		return state
			.split('_')
			.map((part) => part.charAt(0).toUpperCase() + part.slice(1))
			.join(' ');
	}

	function sleep(ms: number): Promise<void> {
		return new Promise((resolve) => setTimeout(resolve, ms));
	}

	function formatRequestError(error: unknown, fallback: string): string {
		if (error instanceof Error) {
			if (error.name === 'AbortError') {
				return 'sandbox jobs request timed out';
			}
			if (error instanceof SyntaxError || error.message.includes('JSON.parse')) {
				return `${fallback}: received an invalid server response`;
			}
			return error.message;
		}
		return fallback;
	}

	async function refreshRunningSandboxJobs(options?: { manual?: boolean }) {
		if (!orgId) {
			runningSandboxJobs = [];
			runningJobsError = null;
			return;
		}
		if (runningJobsLoading) return;

		const manual = options?.manual ?? false;
		runningJobsLoading = true;
		if (manual) {
			runningJobsManualRefresh = true;
		}
		try {
			const controller = new AbortController();
			const timeoutID = setTimeout(() => controller.abort(), 10000);
			const response = await listSandboxJobs(orgId, {
					state: 'queued,running',
					limit: 25
				}, {
					signal: controller.signal
				});
			clearTimeout(timeoutID);

			if (response.status !== 200) {
				runningJobsError =
					(response.data as { detail?: string })?.detail ?? 'failed to load running jobs';
				return;
			}

			runningSandboxJobs = response.data.jobs ?? [];
			runningJobsError = null;
		} catch (err) {
			runningJobsError = formatRequestError(err, 'failed to load running jobs');
		} finally {
			runningJobsLoading = false;
			if (manual) {
				runningJobsManualRefresh = false;
			}
		}
	}

	async function openSandboxJob(jobID: string) {
		if (!jobID) return;
		sandboxError = null;
		sandboxLogs = [];
		sandboxLogsCursor = 0;
		sandboxJob = null;
		await pollSandboxJob(jobID);
	}

	async function pollSandboxJob(jobID: string) {
		if (!orgId || sandboxPolling) return;
		sandboxPolling = true;

		try {
			while (true) {
				const jobResp = await getSandboxJob(orgId, jobID);
				if (jobResp.status !== 200) {
					sandboxError =
						(jobResp.data as { detail?: string })?.detail ?? 'failed to fetch sandbox job';
					break;
				}
				sandboxJob = jobResp.data;

				const logsResp = await getSandboxJobLogs(orgId, jobID, {
					cursor: sandboxLogsCursor,
					limit: 200
				});
				if (logsResp.status === 200) {
					const nextLogs = logsResp.data.logs ?? [];
					if (nextLogs.length > 0) {
						sandboxLogs = [...sandboxLogs, ...nextLogs];
					}
					sandboxLogsCursor = logsResp.data.next_cursor ?? sandboxLogsCursor;
				}

				if (
					sandboxJob.state === 'succeeded' ||
					sandboxJob.state === 'failed' ||
					sandboxJob.state === 'cancelled' ||
					sandboxJob.state === 'timed_out'
				) {
					break;
				}

				await sleep(2000);
			}
		} catch (err) {
			sandboxError = err instanceof Error ? err.message : 'sandbox polling failed';
		} finally {
			sandboxPolling = false;
		}
	}

	async function startSandboxJob() {
		if (!orgId || !sandboxRepositoryID || sandboxSubmitting) return;
		sandboxSubmitting = true;
		sandboxError = null;
		sandboxLogs = [];
		sandboxLogsCursor = 0;
		sandboxJob = null;

		const commandLines = sandboxCommands
			.split('\n')
			.map((line) => line.trim())
			.filter((line) => line.length > 0);
		if (commandLines.length === 0) {
			sandboxError = 'at least one command is required';
			sandboxSubmitting = false;
			return;
		}

		try {
			const response = await createSandboxJob(orgId, {
				repository_id: sandboxRepositoryID,
				base_branch: undefined,
				task_title: sandboxTaskTitle.trim(),
				commit_message: sandboxCommitMessage.trim(),
				commands: commandLines,
				draft_pr: true
			});
			if (response.status !== 200) {
				sandboxError =
					(response.data as { detail?: string })?.detail ?? 'failed to start sandbox job';
				return;
			}

			await pollSandboxJob(response.data.job_id);
			await refreshRunningSandboxJobs();
		} catch (err) {
			sandboxError = err instanceof Error ? err.message : 'failed to start sandbox job';
		} finally {
			sandboxSubmitting = false;
		}
	}

	async function runTestIntegration() {
		if (!orgId || sandboxSubmitting || sandboxPolling) return;
		const targetRepoID = sandboxRepositoryID || enabledGithubRepositories[0]?.id;
		if (!targetRepoID) {
			sandboxError = 'enable at least one repository before running test integration';
			return;
		}

		sandboxSubmitting = true;
		sandboxError = null;
		sandboxLogs = [];
		sandboxLogsCursor = 0;
		sandboxJob = null;

		try {
			const response = await createSandboxJob(orgId, {
				repository_id: targetRepoID,
				base_branch: undefined,
				task_title: 'Test integration',
				commit_message: 'test: add I_WAS_HERE_SESSION_ID',
				commands: [
					'printf "session=%s\\nrun=%s\\n" "$(hostname)" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" > I_WAS_HERE_SESSION_ID'
				],
				draft_pr: true
			});
			if (response.status !== 200) {
				sandboxError =
					(response.data as { detail?: string })?.detail ?? 'failed to run test integration';
				return;
			}

			await pollSandboxJob(response.data.job_id);
			await refreshRunningSandboxJobs();
		} catch (err) {
			sandboxError = err instanceof Error ? err.message : 'failed to run test integration';
		} finally {
			sandboxSubmitting = false;
		}
	}

	async function requestSandboxCancel() {
		if (!orgId || !sandboxJob) return;
		await cancelSandboxJob(orgId, sandboxJob.id);
		await refreshRunningSandboxJobs();
	}
</script>

<PageContainer breadcrumbs={[{ title: 'Settings', href: '/settings' }]}>
	<div class="flex h-full flex-col gap-6">
		<!-- Header -->
		<div class="flex items-start justify-between">
			<div class="flex items-center gap-3">
				<div class="rounded-lg bg-primary/10 p-2">
					<SettingsIcon class="size-6 text-primary" />
				</div>
				<div>
					<h1 class="text-xl font-semibold">Settings</h1>
					<p class="text-sm text-muted-foreground">
						Manage your organization settings for {orgName}
					</p>
				</div>
			</div>
		</div>

		<Separator />

		<!-- GitHub Integration -->
		<Card.Root>
			<Card.Header class="flex flex-row items-center justify-between">
				<div class="flex items-center gap-2">
					<GithubIcon class="size-5 text-muted-foreground" />
					<div>
						<Card.Title>GitHub Integration</Card.Title>
						<Card.Description>
							Install the Orvo GitHub App, sync repositories, and enable repositories for automation
						</Card.Description>
					</div>
				</div>
				<div class="flex items-center gap-2">
					<Button
						variant="outline"
						onclick={syncGithubRepositories}
						disabled={githubSyncing || !orgId}
					>
						{#if githubSyncing}
							<LoaderCircleIcon class="mr-2 size-4 animate-spin" />
							Syncing...
						{:else}
							<RefreshCwIcon class="mr-2 size-4" />
							Sync repositories
						{/if}
					</Button>
					<Button onclick={connectGithub} disabled={connectGithubMutation.isPending || !orgId}>
						{#if connectGithubMutation.isPending}
							Connecting...
						{:else}
							Connect GitHub
						{/if}
					</Button>
				</div>
			</Card.Header>
			<Card.Content class="space-y-4">
				{#if githubConnectState === 'connected'}
					<div
						class="rounded-md border border-emerald-500/40 bg-emerald-500/10 px-3 py-2 text-sm text-emerald-700"
					>
						GitHub installation connected successfully.
					</div>
				{:else if githubConnectState === 'error'}
					<div
						class="rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive"
					>
						GitHub connection failed: {githubConnectCode ?? 'unknown_error'}
					</div>
				{/if}

				<div class="space-y-2">
					<h3 class="text-sm font-medium">Installations</h3>
					{#if githubInstallationsQuery.isPending}
						<div class="text-sm text-muted-foreground">Loading installations...</div>
					{:else if githubInstallations.length === 0}
						<div class="text-sm text-muted-foreground">No GitHub installations connected yet.</div>
					{:else}
						<div class="divide-y rounded-lg border">
							{#each githubInstallations as installation (installation.id)}
								<div class="flex items-center justify-between px-4 py-3">
									<div class="flex flex-col">
										<span class="font-medium">{installation.account_login}</span>
										<span class="text-xs text-muted-foreground">
											Installation #{installation.github_installation_id}
										</span>
									</div>
									<Badge variant={installation.active ? 'secondary' : 'outline'}>
										{installation.active ? 'Active' : 'Inactive'}
									</Badge>
								</div>
							{/each}
						</div>
					{/if}
				</div>

				<div class="space-y-2">
					<h3 class="text-sm font-medium">Repositories</h3>
					{#if githubRepositoriesQuery.isPending}
						<div class="text-sm text-muted-foreground">Loading repositories...</div>
					{:else if githubRepositories.length === 0}
						<div class="text-sm text-muted-foreground">
							No repositories found. Install the app on at least one repository or organization.
						</div>
					{:else}
						<div class="divide-y rounded-lg border">
							{#each githubRepositories as repo (repo.id)}
								<div class="flex items-center justify-between px-4 py-3">
									<div class="flex flex-col gap-1">
										<div class="flex items-center gap-2">
											<span class="font-medium">{repo.full_name}</span>
											{#if repo.archived}
												<Badge variant="outline">Archived</Badge>
											{/if}
										</div>
										<span class="text-xs text-muted-foreground"
											>Default branch: {repo.default_branch}</span
										>
									</div>
									<Button
										variant={repo.enabled ? 'secondary' : 'outline'}
										size="sm"
										onclick={() => toggleGithubRepositoryEnabled(repo)}
										disabled={setGithubRepoEnabledMutation.isPending}
									>
										{repo.enabled ? 'Enabled' : 'Enable'}
									</Button>
								</div>
							{/each}
						</div>
					{/if}
				</div>
			</Card.Content>
		</Card.Root>

		<Card.Root id="remediation-mappings">
			<Card.Header>
				<Card.Title>Auto Resolve Service Mappings</Card.Title>
				<Card.Description>
					Map telemetry service names to enabled repositories for one-click Auto Resolve from logs.
				</Card.Description>
			</Card.Header>
			<Card.Content class="space-y-4">
				{#if remediationLoading}
					<div class="text-sm text-muted-foreground">Loading service mappings...</div>
				{:else if remediationServices.length === 0}
					<div class="text-sm text-muted-foreground">
						No services discovered yet. Send logs first, then map services to repositories.
					</div>
				{:else}
					<div class="divide-y rounded-lg border">
						{#each remediationServices as serviceName (serviceName)}
							{@const existingMapping = remediationMappings.find(
								(mapping) => mapping.service_name === serviceName
							)}
							<div class="grid gap-3 px-4 py-3 md:grid-cols-[1fr_1fr_auto_auto] md:items-center">
								<div class="text-sm font-medium">{serviceName}</div>
								<select
									class="h-9 rounded-md border border-input bg-background px-3 text-sm"
									value={remediationRepoByService[serviceName] ?? ''}
									onchange={(event) => {
										const repositoryID = (event.currentTarget as HTMLSelectElement).value;
										remediationRepoByService = {
											...remediationRepoByService,
											[serviceName]: repositoryID
										};
									}}
									disabled={enabledGithubRepositories.length === 0}
								>
									{#if enabledGithubRepositories.length === 0}
										<option value="" disabled>Enable a repository first</option>
									{:else}
										{#each enabledGithubRepositories as repo (repo.id)}
											<option value={repo.id}>{repo.full_name}</option>
										{/each}
									{/if}
								</select>
								<Button
									size="sm"
									onclick={() => saveRemediationMapping(serviceName)}
									disabled={
										enabledGithubRepositories.length === 0 ||
										remediationSavingService === serviceName ||
										!remediationRepoByService[serviceName]
									}
								>
									{#if remediationSavingService === serviceName}
										Saving...
									{:else if existingMapping}
										Update
									{:else}
										Save
									{/if}
								</Button>
								<Button
									size="sm"
									variant="outline"
									onclick={() => removeRemediationMapping(serviceName)}
									disabled={!existingMapping || remediationDeletingService === serviceName}
								>
									{#if remediationDeletingService === serviceName}
										Removing...
									{:else}
										Remove
									{/if}
								</Button>
								{#if existingMapping}
									<div class="text-xs text-muted-foreground md:col-span-4">
										Mapped to {existingMapping.repository_full_name}
									</div>
								{/if}
							</div>
						{/each}
					</div>
				{/if}

				{#if remediationError}
					<div
						class="rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive"
					>
						{remediationError}
					</div>
				{/if}
			</Card.Content>
		</Card.Root>

		<!-- Sandbox Automation -->
		<Card.Root>
			<Card.Header class="flex flex-row items-center justify-between">
				<div class="flex items-center gap-2">
					<GitBranchIcon class="size-5 text-muted-foreground" />
					<div>
						<Card.Title>Sandbox Automation</Card.Title>
						<Card.Description>
							Run commands in an isolated sandbox and create a draft pull request from enabled
							repositories
						</Card.Description>
					</div>
				</div>
				<Button
					variant="outline"
					onclick={runTestIntegration}
					disabled={enabledGithubRepositories.length === 0 || sandboxSubmitting || sandboxPolling}
				>
					Test Integration
				</Button>
			</Card.Header>
			<Card.Content class="space-y-4">
				{#if enabledGithubRepositories.length === 0}
					<div class="text-sm text-muted-foreground">
						Enable at least one GitHub repository above to run sandbox jobs.
					</div>
				{:else}
					<div class="grid gap-3 md:grid-cols-2">
						<div class="flex flex-col gap-2">
							<label class="text-sm font-medium" for="sandbox-repo">Repository</label>
							<select
								id="sandbox-repo"
								class="h-9 rounded-md border border-input bg-background px-3 text-sm"
								bind:value={sandboxRepositoryID}
							>
								<option value="" disabled>Select repository</option>
								{#each enabledGithubRepositories as repo (repo.id)}
									<option value={repo.id}>{repo.full_name}</option>
								{/each}
							</select>
						</div>
						<div class="flex flex-col gap-2">
							<label class="text-sm font-medium" for="sandbox-title">Task title</label>
							<Input id="sandbox-title" bind:value={sandboxTaskTitle} />
						</div>
					</div>

					<div class="flex flex-col gap-2">
						<label class="text-sm font-medium" for="sandbox-commit">Commit message</label>
						<Input id="sandbox-commit" bind:value={sandboxCommitMessage} />
					</div>

					<div class="flex flex-col gap-2">
						<label class="text-sm font-medium" for="sandbox-commands">Commands (one per line)</label
						>
						<textarea
							id="sandbox-commands"
							class="min-h-28 rounded-md border border-input bg-background px-3 py-2 font-mono text-sm"
							bind:value={sandboxCommands}
						></textarea>
					</div>

					<div class="flex items-center gap-2">
						<Button
							onclick={startSandboxJob}
							disabled={!sandboxRepositoryID || sandboxSubmitting || sandboxPolling}
						>
							{#if sandboxSubmitting || sandboxPolling}
								<LoaderCircleIcon class="mr-2 size-4 animate-spin" />
								Running...
							{:else}
								<PlayIcon class="mr-2 size-4" />
								Run in Sandbox
							{/if}
						</Button>

						{#if sandboxJob && (sandboxJob.state === 'queued' || sandboxJob.state === 'running')}
							<Button variant="outline" onclick={requestSandboxCancel}>Cancel</Button>
						{/if}
					</div>

						{#if sandboxError}
							<div
								class="rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive"
							>
								{sandboxError}
							</div>
						{/if}

						<div class="space-y-2">
							<div class="flex items-center justify-between">
								<div class="text-sm font-medium">Running Sandbox Jobs</div>
									<Button
										variant="outline"
										size="sm"
										onclick={() => refreshRunningSandboxJobs({ manual: true })}
										disabled={runningJobsManualRefresh}
									>
										{#if runningJobsManualRefresh}
											<LoaderCircleIcon class="mr-1 size-3 animate-spin" />
											Refreshing...
										{:else}
											Refresh
									{/if}
								</Button>
							</div>

							{#if runningJobsError}
								<div class="rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-xs text-destructive">
									{runningJobsError}
								</div>
							{:else if runningSandboxJobs.length === 0}
								<div class="rounded-md border bg-muted/30 px-3 py-2 text-xs text-muted-foreground">
									No queued or running jobs.
								</div>
							{:else}
								<div class="divide-y rounded-md border">
									{#each runningSandboxJobs as runningJob (runningJob.id)}
										<div class="flex items-center justify-between gap-3 px-3 py-2 text-xs">
											<div class="min-w-0 space-y-1">
												<div class="font-mono text-sm">{runningJob.id}</div>
												<div class="text-muted-foreground">
													{repoNameByID[runningJob.repository_id] ?? runningJob.repository_id} |
													{runningJob.mode || 'manual'} | created {formatRelative(runningJob.created_at)}
												</div>
											</div>
											<div class="flex items-center gap-2">
												<Badge variant="secondary">{formatSandboxState(runningJob.state)}</Badge>
												<Button
													variant="outline"
													size="sm"
													onclick={() => openSandboxJob(runningJob.id)}
													disabled={sandboxPolling}
												>
													Open
												</Button>
											</div>
										</div>
									{/each}
								</div>
							{/if}
						</div>

						{#if sandboxJob}
							<div class="rounded-md border p-3">
							<div class="mb-2 flex items-center justify-between">
								<span class="text-sm font-medium">Job {sandboxJob.id}</span>
								<Badge variant="secondary">{formatSandboxState(sandboxJob.state)}</Badge>
							</div>
							<div class="space-y-1 text-xs text-muted-foreground">
								<div>Branch: {sandboxJob.branch_name || 'pending'}</div>
								{#if sandboxJob.pull_request_url}
									<div>
										Pull Request:
										<a
											class="text-primary underline"
											href={sandboxJob.pull_request_url}
											target="_blank"
											rel="noreferrer"
										>
											{sandboxJob.pull_request_url}
										</a>
									</div>
								{/if}
							</div>

								<div class="mt-3 max-h-60 overflow-auto rounded-md bg-muted p-2 font-mono text-xs">
									{#if sandboxLogs.length === 0}
										<div class="text-muted-foreground">
											{#if sandboxJob.state === 'running' || sandboxJob.state === 'queued'}
												Job is active. Waiting for first log line...
											{:else}
												No logs yet.
											{/if}
										</div>
									{:else}
										{#each sandboxLogs as log (log.seq)}
											<div class="break-all whitespace-pre-wrap">
											[{log.stream}] {log.message}
										</div>
									{/each}
								{/if}
							</div>
						</div>
					{/if}
				{/if}
			</Card.Content>
		</Card.Root>

		<!-- API Keys Section -->
		<Card.Root>
			<Card.Header class="flex flex-row items-center justify-between">
				<div class="flex items-center gap-2">
					<KeyIcon class="size-5 text-muted-foreground" />
					<div>
						<Card.Title>API Keys</Card.Title>
						<Card.Description>
							Create and manage API keys for sending logs via OTLP
						</Card.Description>
					</div>
				</div>
				<Dialog.Root
					bind:open={createDialogOpen}
					onOpenChange={(open) => {
						if (!open) closeCreateDialog();
					}}
				>
					<Dialog.Trigger>
						{#snippet child({ props })}
							<Button size="sm" {...props}>
								<PlusIcon class="mr-1 size-4" />
								Create key
							</Button>
						{/snippet}
					</Dialog.Trigger>
					<Dialog.Content class="sm:max-w-md">
						{#if createdKeyValue}
							<!-- Show created key -->
							<Dialog.Header>
								<Dialog.Title>API Key Created</Dialog.Title>
								<Dialog.Description>
									Copy this key now. You won't be able to see it again.
								</Dialog.Description>
							</Dialog.Header>
							<div class="flex items-center gap-2">
								<Input value={createdKeyValue} readonly class="font-mono text-sm" />
								<Button variant="outline" size="icon" onclick={copyKey}>
									{#if copiedKey}
										<CheckIcon class="size-4 text-green-500" />
									{:else}
										<CopyIcon class="size-4" />
									{/if}
								</Button>
							</div>
							<Dialog.Footer>
								<Button onclick={closeCreateDialog}>Done</Button>
							</Dialog.Footer>
						{:else}
							<!-- Create key form -->
							<Dialog.Header>
								<Dialog.Title>Create API Key</Dialog.Title>
								<Dialog.Description>
									API keys are used to authenticate log ingestion via OTLP.
								</Dialog.Description>
							</Dialog.Header>
							<div class="flex flex-col gap-4">
								<div class="flex flex-col gap-2">
									<label for="key-name" class="text-sm font-medium">Name</label>
									<Input
										id="key-name"
										bind:value={newKeyName}
										placeholder="e.g. production-ingest"
									/>
								</div>
								<div class="flex flex-col gap-2">
									<label for="key-expiry" class="text-sm font-medium">Expiration</label>
									<Select.Root type="single" bind:value={newKeyExpiry}>
										<Select.Trigger>
											<span>
												{expiryOptions.find((o) => o.value === newKeyExpiry)?.label ?? 'Never'}
											</span>
										</Select.Trigger>
										<Select.Content>
											{#each expiryOptions as opt (opt.value)}
												<Select.Item value={opt.value}>{opt.label}</Select.Item>
											{/each}
										</Select.Content>
									</Select.Root>
								</div>
							</div>
							<Dialog.Footer>
								<Button variant="outline" onclick={closeCreateDialog}>Cancel</Button>
								<Button
									onclick={handleCreateKey}
									disabled={!newKeyName.trim() || createKeyMutation.isPending}
								>
									{#if createKeyMutation.isPending}
										Creating...
									{:else}
										Create
									{/if}
								</Button>
							</Dialog.Footer>
						{/if}
					</Dialog.Content>
				</Dialog.Root>
			</Card.Header>
			<Card.Content>
				{#if apiKeysQuery.isPending}
					<div class="flex h-24 items-center justify-center text-sm text-muted-foreground">
						Loading API keys...
					</div>
				{:else if apiKeys.length === 0}
					<div
						class="flex h-24 flex-col items-center justify-center gap-2 text-sm text-muted-foreground"
					>
						<KeyIcon class="size-8 opacity-50" />
						<p>No API keys yet. Create one to start sending logs.</p>
					</div>
				{:else}
					<div class="divide-y rounded-lg border">
						{#each apiKeys as key (key.id)}
							<div class="flex items-center justify-between px-4 py-3">
								<div class="flex flex-col gap-1">
									<div class="flex items-center gap-2">
										<span class="font-medium">{key.name}</span>
										{#if key.expires_at}
											{@const expired = new Date(key.expires_at) < new Date()}
											<Badge variant={expired ? 'destructive' : 'secondary'}>
												{expired ? 'Expired' : `Expires ${formatDate(key.expires_at)}`}
											</Badge>
										{:else}
											<Badge variant="outline">No expiration</Badge>
										{/if}
									</div>
									<div class="flex items-center gap-3 text-xs text-muted-foreground">
										<span>Created {formatDate(key.created_at)}</span>
										<span>Last used: {formatRelative(key.last_used_at)}</span>
									</div>
								</div>
								<Button
									variant="ghost"
									size="icon"
									class="text-muted-foreground hover:text-destructive"
									onclick={() => confirmRevoke(key)}
								>
									<TrashIcon class="size-4" />
								</Button>
							</div>
						{/each}
					</div>
				{/if}
			</Card.Content>
		</Card.Root>

		<!-- Revoke Confirmation Dialog -->
		<Dialog.Root bind:open={revokeDialogOpen}>
			<Dialog.Content class="sm:max-w-md">
				<Dialog.Header>
					<Dialog.Title>Revoke API Key</Dialog.Title>
					<Dialog.Description>
						Are you sure you want to revoke <strong>{keyToRevoke?.name}</strong>? Any services using
						this key will immediately lose access.
					</Dialog.Description>
				</Dialog.Header>
				<Dialog.Footer>
					<Button variant="outline" onclick={() => (revokeDialogOpen = false)}>Cancel</Button>
					<Button
						variant="destructive"
						onclick={handleRevoke}
						disabled={revokeKeyMutation.isPending}
					>
						{#if revokeKeyMutation.isPending}
							Revoking...
						{:else}
							Revoke key
						{/if}
					</Button>
				</Dialog.Footer>
			</Dialog.Content>
		</Dialog.Root>
	</div>
</PageContainer>
