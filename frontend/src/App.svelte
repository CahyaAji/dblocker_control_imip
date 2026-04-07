<script lang="ts">
  import LoginPage from "./lib/components/LoginPage.svelte";
  import Map from "./lib/components/Map.svelte";
  import MsgReceiverBox from "./lib/components/MsgReceiverBox.svelte";
  import MsgStatusBox from "./lib/components/MsgStatusBox.svelte";
  import SideMenu from "./lib/components/SideMenu.svelte";
  import UserManagement from "./lib/components/UserManagement.svelte";
  import { authStore, logout, verifyToken } from "./lib/store/authStore";
  import { settings } from "./lib/store/configStore";
  import { startPolling, stopPolling } from "./lib/store/dblockerStore";
  import {
    startDetectorPolling,
    stopDetectorPolling,
  } from "./lib/store/detectorStore";

  let isResizing = $state(false);
  let activeMsgTab = $state<"receiver" | "status">("receiver");
  let showUserMgmt = $state(false);

  // Verify token on mount
  $effect(() => {
    if ($authStore.token) {
      verifyToken();
    }
  });

  $effect(() => {
    if ($authStore.token) {
      startPolling(2000);
      startDetectorPolling(10000);
      return () => {
        stopPolling();
        stopDetectorPolling();
      };
    }
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

{#if !$authStore.token}
  <LoginPage />
{:else}
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
              <div class="header-actions">
                {#if $authStore.user?.is_admin}
                  <button
                    class="icon-btn"
                    onclick={() => (showUserMgmt = !showUserMgmt)}
                    aria-label="User Management"
                    title="User Management"
                  >
                    <svg
                      xmlns="http://www.w3.org/2000/svg"
                      width="18"
                      height="18"
                      viewBox="0 0 24 24"
                      fill="none"
                      stroke="currentColor"
                      stroke-width="2"
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      ><path
                        d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"
                      /><circle cx="9" cy="7" r="4" /><path
                        d="M23 21v-2a4 4 0 0 0-3-3.87"
                      /><path d="M16 3.13a4 4 0 0 1 0 7.75" /></svg
                    >
                  </button>
                {/if}
                <a
                  class="icon-btn"
                  href="/detections"
                  aria-label="Drone Detections"
                  title="Drone Detections"
                >
                  <svg
                    xmlns="http://www.w3.org/2000/svg"
                    width="18"
                    height="18"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="2"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    ><circle cx="12" cy="12" r="10" /><circle
                      cx="12"
                      cy="12"
                      r="6"
                    /><circle cx="12" cy="12" r="2" /><line
                      x1="12"
                      y1="2"
                      x2="12"
                      y2="4"
                    /><line x1="12" y1="20" x2="12" y2="22" /><line
                      x1="2"
                      y1="12"
                      x2="4"
                      y2="12"
                    /><line x1="20" y1="12" x2="22" y2="12" /></svg
                  >
                </a>
                <a
                  class="icon-btn"
                  href="/logs"
                  aria-label="Action Logs"
                  title="Action Logs"
                >
                  <svg
                    xmlns="http://www.w3.org/2000/svg"
                    width="18"
                    height="18"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="2"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    ><path
                      d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"
                    /><polyline points="14 2 14 8 20 8" /><line
                      x1="16"
                      y1="13"
                      x2="8"
                      y2="13"
                    /><line x1="16" y1="17" x2="8" y2="17" /><polyline
                      points="10 9 9 9 8 9"
                    /></svg
                  >
                </a>
                <button
                  class="icon-btn"
                  onclick={logout}
                  aria-label="Logout"
                  title="Logout ({$authStore.user?.username})"
                >
                  <svg
                    xmlns="http://www.w3.org/2000/svg"
                    width="18"
                    height="18"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="2"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    ><path
                      d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"
                    /><polyline points="16 17 21 12 16 7" /><line
                      x1="21"
                      y1="12"
                      x2="9"
                      y2="12"
                    /></svg
                  >
                </button>
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
              </div>
            {/if}
          </div>
          <div class="sidebar-content">
            {#if $settings.sidebarExpanded}
              {#if showUserMgmt && $authStore.user?.is_admin}
                <UserManagement />
              {:else}
                <SideMenu />
              {/if}
            {/if}
          </div>
        </aside>
      </div>
    </main>
  </div>

  <div class="dev-panel msg-panel" role="region" aria-label="Message panel">
    <div class="msg-tabs" role="tablist" aria-label="Message tabs">
      <button
        type="button"
        class="msg-tab"
        class:active={activeMsgTab === "receiver"}
        role="tab"
        aria-selected={activeMsgTab === "receiver"}
        onclick={() => (activeMsgTab = "receiver")}
      >
        Receiver
      </button>
      <button
        type="button"
        class="msg-tab"
        class:active={activeMsgTab === "status"}
        role="tab"
        aria-selected={activeMsgTab === "status"}
        onclick={() => (activeMsgTab = "status")}
      >
        Status
      </button>
    </div>

    <div class="msg-tab-content">
      {#if activeMsgTab === "receiver"}
        <!-- For testing only -->
        <!-- <SubDemo /> -->
        <MsgReceiverBox />
      {:else}
        <MsgStatusBox />
      {/if}
    </div>
  </div>
{/if}

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

  .msg-panel {
    left: 14px;
    bottom: 14px;
    max-height: min(75vh, 480px);
  }

  .msg-tabs {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 6px;
  }

  .msg-tab {
    border: 1px solid color-mix(in srgb, var(--separator) 72%, transparent);
    background: color-mix(in srgb, var(--card-bg) 88%, var(--bg-elevated) 12%);
    color: var(--text-secondary);
    border-radius: 8px;
    padding: 6px 8px;
    font-size: 12px;
    cursor: pointer;
    transition: all 0.2s ease;
  }

  .msg-tab:hover {
    color: var(--text-primary);
    border-color: color-mix(in srgb, var(--accent-blue) 38%, transparent);
  }

  .msg-tab.active {
    color: var(--text-primary);
    border-color: color-mix(in srgb, var(--accent-cyan) 60%, transparent);
    background: color-mix(in srgb, var(--accent-cyan) 16%, var(--card-bg) 84%);
    box-shadow: inset 0 0 0 1px
      color-mix(in srgb, var(--accent-cyan) 24%, transparent);
  }

  .msg-tab-content {
    overflow: auto;
    min-height: 0;
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

  .header-actions {
    display: flex;
    align-items: center;
    gap: 6px;
  }

  .icon-btn {
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

  .icon-btn:hover {
    transform: translateY(-1px);
    border-color: color-mix(in srgb, var(--accent-blue) 40%, transparent);
    box-shadow: 0 6px 16px rgba(19, 134, 217, 0.18);
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
