<script setup>
import AppNav from './components/AppNav.vue'
import AppFooter from './components/AppFooter.vue'
</script>

<template>
  <AppNav />
  <main class="page">
    <RouterView />
  </main>
  <AppFooter />
</template>
