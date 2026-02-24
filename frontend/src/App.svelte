<script lang="ts">
  import Map from "./lib/components/Map.svelte";
  import MsgReceiverBox from "./lib/components/MsgReceiverBox.svelte";
  import SideMenu from "./lib/components/SideMenu.svelte";
  import { settings } from "./lib/store/configStore";
  import { startPolling, stopPolling } from "./lib/store/dblockerStore";

  let isResizing = $state(false);

  $effect(() => {
    startPolling(2000);
    return () => stopPolling();
  });

  const toggleSidebar = () => {
    $settings.sidebarExpanded = !$settings.sidebarExpanded;
  };

  const toggleTheme = () => {
    $settings.theme = $settings.theme === "dark" ? "light" : "dark";
  };

  $effect(() => {
    if (typeof document !== "undefined") {
      document.documentElement.classList.toggle(
        "dark",
        $settings.theme === "dark",
      );
    }
  });

  const startResize = (e: MouseEvent) => {
    isResizing = true;
    e.preventDefault();
  };

  const handleMouseMove = (e: MouseEvent) => {
    if (!isResizing) return;
    const newWidth = window.innerWidth - e.clientX;

    // Limits: Min 300px, Max 50% of screen
    if (newWidth > 330 && newWidth < window.innerWidth * 0.5) {
      $settings.sidebarWidth = newWidth;
    }
  };

  const stopResize = () => (isResizing = false);
</script>

<svelte:window onmousemove={handleMouseMove} onmouseup={stopResize} />

<div class="app-container">
  <main>
    <div class="map-area">
      <Map />
    </div>

    <div class="sidebar-wrapper">
      {#if $settings.sidebarExpanded}
        <button
          type="button"
          class="resizer"
          class:active={isResizing}
          onmousedown={startResize}
          aria-label="Resize sidebar"
        ></button>
      {/if}

      <aside
        style={$settings.sidebarExpanded
          ? `width: ${$settings.sidebarWidth}px`
          : "width: 50px"}
        class:resizing={isResizing}
      >
        <div class="sidebar-header">
          <button
            class="hamburger"
            onclick={toggleSidebar}
            aria-label="Toggle Sidebar">☰</button
          >
          {#if $settings.sidebarExpanded}
            <button
              class="theme-toggle"
              onclick={toggleTheme}
              aria-label="Toggle Theme"
            >
              {#if $settings.theme === "dark"}
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  width="20"
                  height="20"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="2"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  ><circle cx="12" cy="12" r="5"></circle><line
                    x1="12"
                    y1="1"
                    x2="12"
                    y2="3"
                  ></line><line x1="12" y1="21" x2="12" y2="23"></line><line
                    x1="4.22"
                    y1="4.22"
                    x2="5.64"
                    y2="5.64"
                  ></line><line x1="18.36" y1="18.36" x2="19.78" y2="19.78"
                  ></line><line x1="1" y1="12" x2="3" y2="12"></line><line
                    x1="21"
                    y1="12"
                    x2="23"
                    y2="12"
                  ></line><line x1="4.22" y1="19.78" x2="5.64" y2="18.36"
                  ></line><line x1="18.36" y1="5.64" x2="19.78" y2="4.22"
                  ></line></svg
                >
              {:else}
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  width="20"
                  height="20"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="2"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  ><path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"
                  ></path></svg
                >
              {/if}
            </button>
          {/if}
        </div>
        <div class="sidebar-content">
          {#if $settings.sidebarExpanded}
            <SideMenu />
          {/if}
        </div>
      </aside>
    </div>
  </main>
</div>

<div
  style="position: fixed; bottom: 10px; left: 10px; z-index: 1000; background: rgba(255, 255, 255, 0.8); padding: 8px; border-radius: 4px; box-shadow: 0 2px 4px rgba(0,0,0,0.2);"
>
  <!-- For testing only -->
  <!-- <SubDemo /> -->
  <MsgReceiverBox />
</div>

<style>
  .app-container {
    display: flex;
    flex-direction: column;
    height: 100vh;
    width: 100vw;
    overflow: hidden;
  }

  main {
    display: flex;
    flex: 1;
    overflow: hidden;
    position: relative;
  }

  .map-area {
    flex: 1;
    background-color: #f0f0f0;
    position: relative;
  }

  .sidebar-wrapper {
    display: flex;
    position: relative;
    z-index: 10;
  }

  aside {
    background-color: var(--background-color-light);
    height: 100%;
    display: flex;
    flex-direction: column;
    box-shadow: -2px 0 5px rgba(0, 0, 0, 0.1);
    transition: width 0.3s ease;
  }

  aside.resizing {
    transition: none;
  }

  .sidebar-header {
    display: flex;
    align-items: center;
    justify-content: left;
    padding: 10px;
    border-bottom: 1px solid #eee;
    gap: 10px;
  }

  .hamburger {
    background: transparent;
    border: none;
    font-size: 24px;
    cursor: pointer;
    color: var(--text-primary);
    padding: 5px;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .hamburger:hover {
    background-color: var(--separator);
    border-radius: 4px;
  }

  .theme-toggle {
    background: transparent;
    border: none;
    cursor: pointer;
    color: var(--text-primary);
    padding: 5px;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .theme-toggle:hover {
    background-color: var(--separator);
    border-radius: 4px;
  }

  .sidebar-content {
    padding-right: 4px;
    flex: 1;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  .resizer {
    width: 4px;
    cursor: col-resize;
    background-color: #f0f0f0;
    border-left: 1px solid #ccc;
    transition: background-color 0.2s;
    height: 100%;
    padding: 0;
    border: none;
    background: none;
  }

  .resizer:hover,
  .resizer.active,
  .resizer:focus {
    background-color: #bbb;
  }
</style>
