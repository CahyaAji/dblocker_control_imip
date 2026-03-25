<script lang="ts">
  import Map from "./lib/components/Map.svelte";
  import MsgReceiverBox from "./lib/components/MsgReceiverBox.svelte";
  import MsgStatusBox from "./lib/components/MsgStatusBox.svelte";
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

    // Limits: Min 370px, Max 50% of screen
    if (newWidth > 370 && newWidth < window.innerWidth * 0.5) {
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

<div class="dev-panel msg-1">
  <!-- For testing only -->
  <!-- <SubDemo /> -->
  <MsgReceiverBox />
</div>

<div class="dev-panel msg-2">
  <MsgStatusBox />
</div>

<style>
  .dev-panel {
    position: fixed;
    z-index: 1000;
    display: flex;
    flex-direction: column;
    gap: 8px;
    width: min(420px, calc(100vw - 28px));
    overflow: auto;
    padding: 8px;
    border-radius: 12px;
    background: color-mix(in srgb, var(--card-bg) 85%, transparent);
    border: 1px solid color-mix(in srgb, var(--separator) 78%, transparent);
    box-shadow: var(--shadow-md);
    backdrop-filter: blur(8px);
  }

  .msg-1 {
    left: 14px;
    bottom: 14px;
    max-height: min(75vh, 480px);
  }

  .msg-2 {
    left: 14px;
    top: 100px;
    max-height: 240px;
  }

  .app-container {
    display: flex;
    flex-direction: column;
    height: 100vh;
    width: 100vw;
    overflow: hidden;
    background: transparent;
  }

  main {
    display: flex;
    flex: 1;
    overflow: hidden;
    position: relative;
  }

  .map-area {
    flex: 1;
    background-color: color-mix(
      in srgb,
      var(--bg-elevated) 82%,
      var(--card-bg) 18%
    );
    position: relative;
  }

  .sidebar-wrapper {
    display: flex;
    position: relative;
    z-index: 10;
  }

  aside {
    background: linear-gradient(
      180deg,
      color-mix(in srgb, var(--card-bg) 92%, var(--accent-cyan) 8%) 0%,
      var(--card-bg) 100%
    );
    border-left: 1px solid color-mix(in srgb, var(--separator) 70%, transparent);
    backdrop-filter: blur(8px);
    height: 100%;
    display: flex;
    flex-direction: column;
    box-shadow: -12px 0 34px rgba(0, 0, 0, 0.12);
    transition: width 0.3s ease;
  }

  aside.resizing {
    transition: none;
  }

  .sidebar-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 10px 6px;
    border-bottom: 1px solid
      color-mix(in srgb, var(--separator) 76%, transparent);
    gap: 10px;
  }

  .hamburger {
    background: color-mix(in srgb, var(--card-bg) 86%, var(--bg-elevated) 14%);
    border: 1px solid color-mix(in srgb, var(--separator) 70%, transparent);
    border-radius: 10px;
    font-size: 20px;
    cursor: pointer;
    color: var(--text-primary);
    padding: 6px 8px;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: all 0.2s ease;
  }

  .hamburger:hover {
    transform: translateY(-1px);
    border-color: color-mix(in srgb, var(--accent-blue) 40%, transparent);
    box-shadow: 0 6px 16px rgba(19, 134, 217, 0.18);
  }

  .theme-toggle {
    background: color-mix(in srgb, var(--card-bg) 86%, var(--bg-elevated) 14%);
    border: 1px solid color-mix(in srgb, var(--separator) 70%, transparent);
    border-radius: 10px;
    cursor: pointer;
    color: var(--text-primary);
    padding: 6px;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: all 0.2s ease;
  }

  .theme-toggle:hover {
    transform: translateY(-1px);
    border-color: color-mix(in srgb, var(--accent-cyan) 45%, transparent);
    box-shadow: 0 6px 16px rgba(19, 182, 217, 0.2);
  }

  .sidebar-content {
    padding: 4px 2px 4px;
    flex: 1;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  .resizer {
    width: 8px;
    cursor: col-resize;
    background: linear-gradient(
      180deg,
      transparent 0%,
      var(--separator) 40%,
      transparent 100%
    );
    transition: background-color 0.2s;
    height: 100%;
    padding: 0;
    border: none;
    opacity: 0.35;
  }

  .resizer:hover,
  .resizer.active,
  .resizer:focus {
    opacity: 0.9;
    background: linear-gradient(
      180deg,
      transparent 0%,
      var(--accent-blue) 50%,
      transparent 100%
    );
  }
</style>
