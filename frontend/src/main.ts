import { createApp } from 'vue'
import App from './App.vue'
import NotificationBell from './NotificationBell.vue'
import './style.css'
import './attachments.css'
import './branding.css'

createApp(App).mount('#app')

const restoreCordysBranding = () => {
  document.title = document.title.replaceAll('Cordial CRM', 'Cordys CRM')
  const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT)
  let node: Text | null
  while ((node = walker.nextNode() as Text | null)) {
    node.nodeValue = node.nodeValue
      ?.replaceAll('Cordial CRM · Python + TypeScript', 'Cordys CRM · Copyright © 2026 FIT2CLOUD')
      .replaceAll('Cordial CRM', 'Cordys CRM')
      .replaceAll('Cordial', 'Cordys') || null
  }
}

const mountNotificationBell = () => {
  const actions = document.querySelector('.topbar-actions')
  if (!actions || document.querySelector('#notification-bell')) return
  const host = document.createElement('div')
  host.id = 'notification-bell'
  actions.insertBefore(host, actions.querySelector('.user-button'))
  createApp(NotificationBell).mount(host)
}

new MutationObserver(() => { restoreCordysBranding(); mountNotificationBell() }).observe(document.body, { childList: true, subtree: true })
restoreCordysBranding()
mountNotificationBell()

if ('serviceWorker' in navigator) {
  window.addEventListener('load', () => { navigator.serviceWorker.register('/service-worker.js', { updateViaCache: 'none' }).catch(() => undefined) })
}
