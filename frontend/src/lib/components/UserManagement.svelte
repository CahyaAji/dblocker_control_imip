<script lang="ts">
  import { authFetch, type AuthUser } from "../store/authStore";
  import { API_BASE } from "../utils/api";

  let users = $state<AuthUser[]>([]);
  let newUsername = $state("");
  let newPassword = $state("");
  let newIsAdmin = $state(false);
  let error = $state("");
  let success = $state("");
  let loading = $state(false);

  async function loadUsers() {
    try {
      const res = await authFetch(`${API_BASE}/api/users`);
      if (res.ok) {
        const data = await res.json();
        users = data.data || [];
      }
    } catch {
      console.error("Failed to load users");
    }
  }

  async function createUser(e: Event) {
    e.preventDefault();
    error = "";
    success = "";
    loading = true;

    try {
      const res = await authFetch(`${API_BASE}/api/users`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          username: newUsername,
          password: newPassword,
          is_admin: newIsAdmin,
        }),
      });

      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        error = data.error || "Failed to create user";
      } else {
        success = `User "${newUsername}" created`;
        newUsername = "";
        newPassword = "";
        newIsAdmin = false;
        loadUsers();
      }
    } catch {
      error = "Network error";
    }

    loading = false;
  }

  async function deleteUser(id: number, username: string) {
    if (!confirm(`Delete user "${username}"?`)) return;

    try {
      const res = await authFetch(`${API_BASE}/api/users/${id}`, {
        method: "DELETE",
      });
      if (res.ok) {
        loadUsers();
      }
    } catch {
      console.error("Failed to delete user");
    }
  }

  $effect(() => {
    loadUsers();
  });
</script>

<div class="user-mgmt">
  <h3>User Management</h3>

  <form class="add-user-form" onsubmit={createUser}>
    {#if error}
      <div class="msg error">{error}</div>
    {/if}
    {#if success}
      <div class="msg success">{success}</div>
    {/if}

    <div class="form-row">
      <input
        type="text"
        bind:value={newUsername}
        placeholder="Username"
        required
        disabled={loading}
      />
      <input
        type="password"
        bind:value={newPassword}
        placeholder="Password"
        required
        minlength="4"
        disabled={loading}
      />
    </div>

    <div class="form-row">
      <label class="admin-check">
        <input type="checkbox" bind:checked={newIsAdmin} disabled={loading} />
        Admin
      </label>
      <button type="submit" class="btn-add" disabled={loading}>Add User</button>
    </div>
  </form>

  <div class="user-list">
    {#each users as user (user.id)}
      <div class="user-item">
        <div class="user-info">
          <span class="username">{user.username}</span>
          {#if user.is_admin}
            <span class="badge admin">admin</span>
          {/if}
        </div>
        <button
          class="btn-delete"
          onclick={() => deleteUser(user.id, user.username)}
          aria-label="Delete user"
        >
          &times;
        </button>
      </div>
    {/each}
  </div>
</div>

<style>
  .user-mgmt {
    padding: 8px 0;
  }

  h3 {
    font-size: 14px;
    color: var(--text-primary);
    margin: 0 0 12px;
  }

  .add-user-form {
    margin-bottom: 16px;
  }

  .form-row {
    display: flex;
    gap: 8px;
    margin-bottom: 8px;
    align-items: center;
  }

  .form-row input[type="text"],
  .form-row input[type="password"] {
    flex: 1;
    padding: 8px 10px;
    border: 1px solid var(--separator);
    border-radius: var(--radius-sm);
    background: var(--bg-elevated);
    color: var(--text-primary);
    font-size: 13px;
    outline: none;
  }

  .form-row input:focus {
    border-color: var(--accent-cyan);
  }

  .admin-check {
    display: flex;
    align-items: center;
    gap: 4px;
    font-size: 13px;
    color: var(--text-secondary);
    cursor: pointer;
    white-space: nowrap;
  }

  .btn-add {
    padding: 8px 14px;
    border: none;
    border-radius: var(--radius-sm);
    background: var(--accent-cyan);
    color: #fff;
    font-size: 13px;
    font-weight: 600;
    cursor: pointer;
    white-space: nowrap;
    transition: all 0.2s;
  }

  .btn-add:hover:not(:disabled) {
    filter: brightness(1.1);
  }

  .btn-add:disabled {
    opacity: 0.6;
  }

  .user-list {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .user-item {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 8px 10px;
    background: var(--bg-elevated);
    border: 1px solid var(--separator);
    border-radius: var(--radius-sm);
  }

  .user-info {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .username {
    font-size: 13px;
    color: var(--text-primary);
  }

  .badge {
    font-size: 10px;
    padding: 2px 6px;
    border-radius: 6px;
    font-weight: 600;
    text-transform: uppercase;
  }

  .badge.admin {
    background: color-mix(in srgb, var(--accent-cyan) 20%, transparent);
    color: var(--accent-cyan);
    border: 1px solid color-mix(in srgb, var(--accent-cyan) 40%, transparent);
  }

  .btn-delete {
    background: none;
    border: none;
    color: var(--text-secondary);
    font-size: 18px;
    cursor: pointer;
    padding: 0 4px;
    line-height: 1;
    transition: color 0.2s;
  }

  .btn-delete:hover {
    color: #ff4444;
  }

  .msg {
    padding: 8px 10px;
    border-radius: var(--radius-sm);
    font-size: 12px;
    margin-bottom: 8px;
  }

  .msg.error {
    background: color-mix(in srgb, #ff4444 15%, var(--card-bg) 85%);
    color: #ff6666;
    border: 1px solid color-mix(in srgb, #ff4444 30%, transparent);
  }

  .msg.success {
    background: color-mix(in srgb, var(--accent-green) 15%, var(--card-bg) 85%);
    color: var(--accent-green);
    border: 1px solid color-mix(in srgb, var(--accent-green) 30%, transparent);
  }
</style>
