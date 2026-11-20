<script setup>
// Верхняя панель навигации (REQ-INT-002). Счётчик непрочитанных уведомлений
// появится вместе с центром уведомлений (F-09).
import { useAuth } from '../stores/auth.js'
import { t } from '../i18n.js'

const auth = useAuth()
</script>

<template>
  <header class="nav">
    <RouterLink to="/" class="brand">{{ t('app.title') }}</RouterLink>
    <nav v-if="auth.loggedIn" class="nav-links" aria-label="Основная навигация">
      <RouterLink to="/wishlist">{{ t('nav.wishlist') }}</RouterLink>
      <RouterLink to="/search">{{ t('nav.search') }}</RouterLink>
      <RouterLink to="/notifications">{{ t('nav.notifications') }}</RouterLink>
      <RouterLink to="/profile">{{ t('nav.profile') }}</RouterLink>
    </nav>
    <nav v-else class="nav-links" aria-label="Вход">
      <RouterLink to="/login">{{ t('nav.login') }}</RouterLink>
      <RouterLink to="/register">{{ t('nav.register') }}</RouterLink>
    </nav>
  </header>
</template>
