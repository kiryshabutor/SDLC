import { createRouter, createWebHistory } from 'vue-router'
import { useAuth } from './stores/auth.js'

// meta.guest — страницы для гостя; meta.auth — только после входа.
const routes = [
  { path: '/', redirect: '/wishlist' },
  { path: '/login', component: () => import('./views/LoginView.vue'), meta: { guest: true } },
  { path: '/register', component: () => import('./views/RegisterView.vue'), meta: { guest: true } },
  { path: '/forgot-password', component: () => import('./views/ForgotPasswordView.vue'), meta: { guest: true } },
  { path: '/reset-password', component: () => import('./views/ResetPasswordView.vue') },
  { path: '/verify', component: () => import('./views/VerifyEmailView.vue') },
  { path: '/privacy', component: () => import('./views/PrivacyView.vue') },
  { path: '/wishlist', component: () => import('./views/WishlistView.vue'), meta: { auth: true } },
  { path: '/search', component: () => import('./views/SearchView.vue'), meta: { auth: true } },
  { path: '/notifications', component: () => import('./views/NotificationsView.vue'), meta: { auth: true } },
  { path: '/profile', component: () => import('./views/ProfileView.vue'), meta: { auth: true } },
  { path: '/:pathMatch(.*)*', component: () => import('./views/NotFoundView.vue') },
]

export const router = createRouter({ history: createWebHistory(), routes })

router.beforeEach(async (to) => {
  const auth = useAuth()
  if (!auth.loaded) {
    try {
      await auth.load()
    } catch {
      // Сервер недоступен: страница покажет ошибку при своём запросе.
    }
  }
  if (to.meta.auth && !auth.loggedIn) return { path: '/login', query: { next: to.fullPath } }
  if (to.meta.guest && auth.loggedIn) return '/wishlist'
})
