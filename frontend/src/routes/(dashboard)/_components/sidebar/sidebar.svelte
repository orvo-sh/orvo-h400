<script lang="ts" module>
	import GalleryVerticalEndIcon from '@lucide/svelte/icons/gallery-vertical-end';
	import SettingsIcon from '@lucide/svelte/icons/settings';

	import { IconLogs } from '@tabler/icons-svelte';
</script>

<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { logout } from '$lib/api/endpoints/auth/auth';
	import * as Sidebar from '$lib/components/ui/sidebar/index.js';
	import { sessionStore } from '$lib/stores/session';
	import type { ComponentProps } from 'svelte';
	import SidebarNavMain from './sidebar-nav-main.svelte';
	import SidebarNavUser from './sidebar-nav-user.svelte';
	import SidebarTeamSwitcher from './sidebar-team-switcher.svelte';
	let {
		ref = $bindable(null),
		collapsible = 'icon',
		...restProps
	}: ComponentProps<typeof Sidebar.Root> = $props();

	const session = $derived($sessionStore);

	const user = $derived(
		session?.user
			? {
					name: session.user.name,
					email: session.user.email,
					avatar: ''
				}
			: { name: '', email: '', avatar: '' }
	);

	const teams = $derived(
		session?.active_organization
			? [
					{
						name: session.active_organization.name,
						logo: GalleryVerticalEndIcon,
						plan: 'Free'
					}
				]
			: []
	);

	async function handleLogout() {
		await logout();
		sessionStore.set(null);
		goto('/sign-in');
	}
</script>

<Sidebar.Root {collapsible} {...restProps}>
	<Sidebar.Header>
		<SidebarTeamSwitcher {teams} />
	</Sidebar.Header>
	<Sidebar.Content>
		<SidebarNavMain
			items={[
				{
					title: 'Logs',
					url: '/logs',
					icon: IconLogs,
					isActive: page.url.pathname.includes('/logs'),
					items: [
						{ title: 'All Logs', url: '/logs' },
						{ title: 'Saved Views', url: '/logs/views' },
						{ title: 'Sources', url: '/logs/sources' }
					]
				},
				{
					title: 'Settings',
					url: '/settings',
					icon: SettingsIcon,
					isActive: page.url.pathname.includes('/settings'),
					items: [
						{ title: 'API Keys', url: '/settings' }
					]
				}
			]}
		/>
	</Sidebar.Content>
	<Sidebar.Footer>
		<SidebarNavUser {user} onLogout={handleLogout} />
	</Sidebar.Footer>
	<Sidebar.Rail />
</Sidebar.Root>
