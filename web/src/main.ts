import { createApp } from 'vue'
import { createPinia } from 'pinia'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import * as Icons from '@element-plus/icons-vue'

import App from './App.vue'
import router from './router'
import { setupDirectives } from './directives'
import './styles/global.css'

const app = createApp(App)

app.use(createPinia())
app.use(router)
app.use(ElementPlus, { locale: zhCn })

// 注册全部图标组件（骨架阶段简化；可按需引入优化体积）
for (const [name, comp] of Object.entries(Icons)) {
  app.component(name, comp)
}

// 等保三级：注册安全指令（v-perm / v-audit / v-sanitize）
setupDirectives(app)

app.mount('#app')
