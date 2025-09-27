import { createRouter, createWebHistory } from 'vue-router'
import LogViewerView from '../views/LogViewerView.vue'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/',
      name: 'logviewer',
      component: LogViewerView,
    },
  ],
})

export default router
