import { createApp } from 'vue'
import { createRouter, createWebHistory } from 'vue-router'
import 'picocrank/styles.css'
import App from './App.vue'
import Game from './views/Game.vue'
import GameHome from './views/GameHome.vue'
import Buildings from './views/Buildings.vue'
import GameResources from './views/GameResources.vue'
import GameResourceDetail from './views/GameResourceDetail.vue'
import GameBank from './views/GameBank.vue'
import GameMarket from './views/GameMarket.vue'
import GameMessages from './views/GameMessages.vue'

const routes = [
  { path: '/', redirect: '/game' },
  {
    path: '/game',
    name: 'game',
    component: Game,
    children: [
      { path: '', name: 'home', component: GameHome },
      { path: 'buildings', name: 'buildings', component: Buildings },
      { path: 'resources', name: 'resources', component: GameResources },
      { path: 'resources/:id', name: 'resource', component: GameResourceDetail },
      { path: 'bank', name: 'bank', component: GameBank },
      { path: 'market', name: 'market', component: GameMarket },
      { path: 'messages', name: 'messages', component: GameMessages },
    ],
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

const app = createApp(App)
app.use(router)
app.mount('#app')
