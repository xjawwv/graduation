export function useApi() {
  const config = useRuntimeConfig()
  return async function api<T>(path: string, options: Parameters<typeof $fetch<T>>[1] = {}): Promise<T> {
    return await $fetch<T>(path, {
      baseURL: config.public.apiBase,
      credentials: 'include',
      ...options,
    })
  }
}
