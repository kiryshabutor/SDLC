import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import { router } from './router.js'
import { setUnauthorizedHandler } from './api/client.js'
import { useAuth } from './stores/auth.js'
import './style.css'

const app = createApp(App)
app.use(createPinia())
app.use(router)

// Сессия истекла или завершена (выход на другом устройстве, сброс пароля).
setUnauthorizedHandler(() => {
  useAuth().forget()
  const current = router.currentRoute.value
  if (current.meta.auth) router.push({ path: '/login', query: { next: current.fullPath } })
})

app.mount('#app')
