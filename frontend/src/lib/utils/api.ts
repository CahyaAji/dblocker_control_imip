export const PORT =
    import.meta.env.VITE_PORT ?? 8080;

const RAW_API_BASE = import.meta.env.VITE_API_BASE ?? '';

export const API_BASE =
    RAW_API_BASE.replace(/\/+$/, '');