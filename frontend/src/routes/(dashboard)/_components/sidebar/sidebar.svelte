<script lang="ts" module>
	import GalleryVerticalEndIcon from '@lucide/svelte/icons/gallery-vertical-end';
	import SettingsIcon from '@lucide/svelte/icons/settings';

	import { IconLogs, IconRoute, IconChartLine, IconLayoutDashboard, IconCpu, IconServer } from '@tabler/icons-svelte';
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
				title: 'Traces',
				url: '/traces',
				icon: IconRoute,
				isActive: page.url.pathname.includes('/traces'),
				items: [
					{ title: 'All Traces', url: '/traces' },
					{ title: 'Service Map', url: '/traces/service-map' },
					{ title: 'Sources', url: '/traces/sources' }
				]
			},
			{
				title: 'Metrics',
				url: '/metrics',
				icon: IconChartLine,
				isActive: page.url.pathname.includes('/metrics'),
				items: [
					{ title: 'Explorer', url: '/metrics' },
					{ title: 'Services', url: '/metrics/services' }
				]
			},
			{
				title: 'Dashboards',
				url: '/dashboards',
				icon: IconLayoutDashboard,
				isActive: page.url.pathname.includes('/dashboards'),
				items: [
					{ title: 'All Dashboards', url: '/dashboards' }
				]
			},
			{
				title: 'Hosts',
				url: '/hosts',
				icon: IconServer,
				isActive: page.url.pathname.includes('/hosts'),
				items: [{ title: 'Infrastructure', url: '/hosts' }]
			},
			{
				title: 'Sandboxes',
				url: '/sandboxes',
				icon: IconCpu,
				isActive: page.url.pathname.includes('/sandboxes'),
				items: [
					{ title: 'Running Jobs', url: '/sandboxes' }
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
