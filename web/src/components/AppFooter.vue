<script setup>
// Атрибуция источников (REQ-COMP-002, REQ-COMP-003) и ссылка на политику
// обработки персональных данных с каждой страницы (REQ-COMP-001).
import { t } from '../i18n.js'
</script>

<template>
  <footer class="footer">
    <span>{{ t('footer.attribution') }}</span>
    <RouterLink to="/privacy">{{ t('footer.privacy') }}</RouterLink>
  </footer>
</template>
