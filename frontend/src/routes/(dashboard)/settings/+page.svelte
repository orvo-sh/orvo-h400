<script lang="ts">
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
	import {
		createListApiKeys,
		createCreateApiKey,
		createRevokeApiKey,
		getListApiKeysQueryKey
	} from '$lib/api/endpoints/api-keys/api-keys';
	import { sessionStore } from '$lib/stores/session';
	import { useQueryClient } from '@tanstack/svelte-query';
	import type { ApiKey } from '$lib/api/model';

	const orgId = $derived($sessionStore?.active_organization?.id ?? '');
	const orgName = $derived($sessionStore?.active_organization?.name ?? '');
	const queryClient = useQueryClient();

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

	// Create key dialog state
	let createDialogOpen = $state(false);
	let newKeyName = $state('');
	let newKeyExpiry = $state<string>('never');
	let createdKeyValue = $state<string | null>(null);
	let copiedKey = $state(false);

	// Revoke confirmation
	let revokeDialogOpen = $state(false);
	let keyToRevoke = $state<ApiKey | null>(null);

	const expiryOptions = [
		{ value: 'never', label: 'Never' },
		{ value: '30', label: '30 days' },
		{ value: '60', label: '60 days' },
		{ value: '90', label: '90 days' },
		{ value: '365', label: '1 year' }
	];

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
</script>

<PageContainer breadcrumbs={[{ title: 'Settings', href: '/settings' }, { title: 'API Keys' }]}>
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
				<Dialog.Root bind:open={createDialogOpen} onOpenChange={(open) => { if (!open) closeCreateDialog(); }}>
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
					<div class="flex h-24 flex-col items-center justify-center gap-2 text-sm text-muted-foreground">
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
						Are you sure you want to revoke <strong>{keyToRevoke?.name}</strong>? Any services
						using this key will immediately lose access.
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
