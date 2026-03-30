<script lang="ts">
  import { login } from "../store/authStore";

  let username = $state("");
  let password = $state("");
  let error = $state("");
  let loading = $state(false);

  async function handleSubmit(e: Event) {
    e.preventDefault();
    error = "";
    loading = true;

    const result = await login(username, password);
    loading = false;

    if (!result.ok) {
      error = result.error || "Login failed";
    }
  }
</script>

<div class="login-overlay">
  <div class="login-card">
    <div class="login-header">
      <svg
        xmlns="http://www.w3.org/2000/svg"
        width="32"
        height="32"
        viewBox="0 0 24 24"
        fill="none"
        stroke="var(--accent-cyan)"
        stroke-width="2"
        stroke-linecap="round"
        stroke-linejoin="round"
      >
        <rect x="3" y="11" width="18" height="11" rx="2" ry="2"></rect>
        <path d="M7 11V7a5 5 0 0 1 10 0v4"></path>
      </svg>
      <h2>DBlocker Control</h2>
      <p class="subtitle">Sign in to continue</p>
    </div>

    <form onsubmit={handleSubmit}>
      {#if error}
        <div class="error-msg">{error}</div>
      {/if}

      <div class="field">
        <label for="username">Username</label>
        <input
          id="username"
          type="text"
          bind:value={username}
          placeholder="Enter username"
          autocomplete="username"
          required
          disabled={loading}
        />
      </div>

      <div class="field">
        <label for="password">Password</label>
        <input
          id="password"
          type="password"
          bind:value={password}
          placeholder="Enter password"
          autocomplete="current-password"
          required
          disabled={loading}
        />
      </div>

      <button type="submit" class="btn-login" disabled={loading}>
        {#if loading}
          Signing in...
        {:else}
          Sign In
        {/if}
      </button>
    </form>
  </div>
</div>

<style>
  .login-overlay {
    position: fixed;
    inset: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--bg-color);
    z-index: 9999;
  }

  .login-card {
    width: min(400px, calc(100vw - 32px));
    background: var(--card-bg);
    border: 1px solid var(--separator);
    border-radius: var(--radius-lg);
    padding: 32px 28px;
    box-shadow: var(--shadow-md);
  }

  .login-header {
    text-align: center;
    margin-bottom: 24px;
  }

  .login-header h2 {
    margin: 12px 0 4px;
    font-size: 20px;
    color: var(--text-primary);
  }

  .subtitle {
    color: var(--text-secondary);
    font-size: 13px;
    margin: 0;
  }

  .field {
    margin-bottom: 16px;
  }

  .field label {
    display: block;
    font-size: 13px;
    font-weight: 500;
    color: var(--text-secondary);
    margin-bottom: 6px;
  }

  .field input {
    width: 100%;
    padding: 10px 12px;
    border: 1px solid var(--separator);
    border-radius: var(--radius-sm);
    background: var(--bg-elevated);
    color: var(--text-primary);
    font-size: 14px;
    outline: none;
    transition: border-color 0.2s;
    box-sizing: border-box;
  }

  .field input:focus {
    border-color: var(--accent-cyan);
    box-shadow: 0 0 0 3px color-mix(in srgb, var(--accent-cyan) 20%, transparent);
  }

  .field input:disabled {
    opacity: 0.6;
  }

  .btn-login {
    width: 100%;
    padding: 11px;
    margin-top: 4px;
    border: none;
    border-radius: var(--radius-sm);
    background: var(--accent-cyan);
    color: #fff;
    font-size: 14px;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.2s;
  }

  .btn-login:hover:not(:disabled) {
    filter: brightness(1.1);
    transform: translateY(-1px);
    box-shadow: 0 4px 12px color-mix(in srgb, var(--accent-cyan) 40%, transparent);
  }

  .btn-login:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }

  .error-msg {
    background: color-mix(in srgb, #ff4444 15%, var(--card-bg) 85%);
    color: #ff6666;
    padding: 10px 12px;
    border-radius: var(--radius-sm);
    font-size: 13px;
    margin-bottom: 16px;
    border: 1px solid color-mix(in srgb, #ff4444 30%, transparent);
  }
</style>
