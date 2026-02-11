<script lang="ts">
    import * as Breadcrumb from "$lib/components/ui/breadcrumb/index.js";
    import { Separator } from "$lib/components/ui/separator/index.js";
    import * as UISidebar from "$lib/components/ui/sidebar/index.js";
    import type { Snippet } from "svelte";

    let {children, breadcrumbs}:{
        children:Snippet;
        breadcrumbs?: { title: string; href?: string }[];
    } = $props();
</script>

<header
      class="flex h-16 shrink-0 items-center gap-2 transition-[width,height] ease-linear group-has-data-[collapsible=icon]/sidebar-wrapper:h-12"
    >
      <div class="flex items-center gap-2 px-4">
        <UISidebar.Trigger class="-ms-1" />
        <Separator orientation="vertical" class="me-2 data-[orientation=vertical]:h-4" />

        {#if breadcrumbs}
        <Breadcrumb.Root>
          <Breadcrumb.List>
            {#each breadcrumbs as breadcrumb, index (breadcrumb.title)}
              <Breadcrumb.Item class={index === 0 ? "hidden md:block" : ""}>
                {#if breadcrumb.href}
                  <Breadcrumb.Link href={breadcrumb.href}>{breadcrumb.title}</Breadcrumb.Link>
                {:else}
                  <Breadcrumb.Page>{breadcrumb.title}</Breadcrumb.Page>
                {/if}
              </Breadcrumb.Item>
              {#if index < breadcrumbs.length - 1}
                <Breadcrumb.Separator class={index === 0 ? "hidden md:block" : ""} />
              {/if}
            {/each}
          </Breadcrumb.List>
        </Breadcrumb.Root>
        {/if}
      </div>
    </header>
    <main class="flex flex-1 flex-col p-4 pt-0">
        {@render children()}
    </main>

        