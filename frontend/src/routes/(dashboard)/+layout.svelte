<script lang="ts">
  import * as UISidebar from "$lib/components/ui/sidebar/index.js";

  import { Sidebar } from "./_components/sidebar";

  let {children} = $props();
</script>

<UISidebar.Provider>
  <Sidebar />
  <UISidebar.Inset>
    
    {@render children()}
  </UISidebar.Inset>
</UISidebar.Provider>
