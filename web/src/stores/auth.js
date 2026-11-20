// Состояние сессии: текущий пользователь (GET /me) и действия входа/выхода.
import { defineStore } from 'pinia'
import { api, ApiError } from '../api/client.js'

export const useAuth = defineStore('auth', {
  state: () => ({ user: null, loaded: false }),
  getters: {
    loggedIn: (s) => s.user !== null,
  },
  actions: {
    async load() {
      try {
        this.user = await api.me()
      } catch (e) {
        if (!(e instanceof ApiError) || e.status !== 401) throw e
        this.user = null
      } finally {
        this.loaded = true
      }
    },
    async login(email, password) {
      const r = await api.login(email, password)
      this.user = r.user
    },
    async register(form) {
      const r = await api.register(form)
      this.user = r.user
    },
    async logout() {
      try {
        await api.logout()
      } finally {
        this.user = null
      }
    },
    forget() {
      this.user = null
    },
  },
})
