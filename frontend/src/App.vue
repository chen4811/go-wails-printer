<template>
  <div class="app-container">
    <!-- 顶部导航栏 -->
    <header class="header">
      <div class="header-left">
        <svg class="logo" viewBox="0 0 24 24" fill="currentColor">
          <path d="M19 8H5c-1.66 0-3 1.34-3 3v6h4v4h12v-4h4v-6c0-1.66-1.34-3-3-3zm-3 11H8v-5h8v5zm3-7c-.55 0-1-.45-1-1s.45-1 1-1 1 .45 1 1-.45 1-1 1zm-1-9H6v4h12V3z"/>
        </svg>
        <span class="title">打印服务器</span>
      </div>
      <div class="header-right">
        <span :class="['status-tag', status.isRunning ? 'status-running' : 'status-stopped']">
          {{ status.isRunning ? '服务运行中' : '服务已停止' }}
        </span>
        <span class="version">v2.0.0</span>
      </div>
    </header>

    <!-- 主内容区 -->
    <main class="main">
      <!-- 左侧控制面板 -->
      <aside class="sidebar">
        <!-- 服务控制卡片 -->
        <div class="card">
          <div class="card-header">
            <h3 class="card-title">服务控制</h3>
          </div>
          <div class="control-content">
            <div class="form-group">
              <label>服务端口</label>
              <input type="number" class="input" v-model.number="config.port" @change="saveConfig" />
            </div>
            <div class="form-group">
              <label>默认打印机</label>
              <select class="select" v-model="config.defaultPrinter" @change="saveConfig">
                <option value="">自动选择</option>
                <option v-for="p in printers" :key="p.name" :value="p.name">
                  {{ p.name }} {{ p.isDefault ? '(默认)' : '' }}
                </option>
              </select>
            </div>
            <div class="form-group checkbox-group">
              <label>
                <input type="checkbox" v-model="config.autoStart" @change="saveConfig" />
                开机自动启动服务
              </label>
            </div>
            <div class="button-group">
              <button 
                v-if="!status.isRunning" 
                class="btn btn-primary" 
                @click="startServer"
                :disabled="loading"
              >
                <svg viewBox="0 0 24 24" fill="currentColor" width="16" height="16">
                  <path d="M8 5v14l11-7z"/>
                </svg>
                启动服务
              </button>
              <button 
                v-else 
                class="btn btn-danger" 
                @click="stopServer"
                :disabled="loading"
              >
                <svg viewBox="0 0 24 24" fill="currentColor" width="16" height="16">
                  <path d="M6 6h12v12H6z"/>
                </svg>
                停止服务
              </button>
            </div>
          </div>
        </div>

        <!-- 打印机列表卡片 -->
        <div class="card">
          <div class="card-header">
            <h3 class="card-title">打印机列表</h3>
            <button class="btn btn-outline" @click="refreshPrinters" style="padding: 6px 12px; font-size: 12px;">
              刷新
            </button>
          </div>
          <div class="printer-list">
            <div v-if="printers.length === 0" class="empty">
              <span class="empty-icon">🖨️</span>
              <span class="empty-text">未检测到打印机</span>
            </div>
            <div v-else class="printer-item" v-for="p in printers" :key="p.name">
              <div class="printer-icon">🖨️</div>
              <div class="printer-info">
                <div class="printer-name">{{ p.name }}</div>
                <div class="printer-status">{{ p.isDefault ? '默认打印机' : '可用' }}</div>
              </div>
            </div>
          </div>
        </div>

        <!-- 连接信息卡片 -->
        <div class="card" v-if="status.isRunning">
          <div class="card-header">
            <h3 class="card-title">连接信息</h3>
          </div>
          <div class="connection-info">
            <div class="info-item">
              <span class="info-label">WebSocket 地址</span>
              <code class="info-value">ws://localhost:{{ config.port }}/ws</code>
            </div>
            <div class="info-item">
              <span class="info-label">HTTP API 地址</span>
              <code class="info-value">http://localhost:{{ config.port }}/api</code>
            </div>
            <div class="info-item">
              <span class="info-label">当前连接数</span>
              <span class="info-value">{{ status.clients }}</span>
            </div>
          </div>
        </div>
      </aside>

      <!-- 右侧任务列表 -->
      <section class="content">
        <div class="card task-card">
          <div class="card-header">
            <h3 class="card-title">打印任务</h3>
            <button class="btn btn-outline" @click="clearTasks" style="padding: 6px 12px; font-size: 12px;">
              清空记录
            </button>
          </div>
          <div class="task-list">
            <div v-if="tasks.length === 0" class="empty">
              <span class="empty-icon">📋</span>
              <span class="empty-text">暂无打印任务</span>
            </div>
            <table v-else class="table">
              <thead>
                <tr>
                  <th>任务ID</th>
                  <th>类型</th>
                  <th>打印机</th>
                  <th>状态</th>
                  <th>进度</th>
                  <th>时间</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="task in tasks" :key="task.id">
                  <td><code>{{ task.id }}</code></td>
                  <td>{{ getFileTypeLabel(task.type) }}</td>
                  <td>{{ task.printer || '默认' }}</td>
                  <td>
                    <span :class="['status-tag', 'status-' + task.status]">
                      {{ getStatusLabel(task.status) }}
                    </span>
                  </td>
                  <td>
                    <div class="progress-bar">
                      <div class="progress-bar-fill" :style="{ width: task.progress + '%' }"></div>
                    </div>
                  </td>
                  <td>{{ formatTime(task.createdAt) }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </section>
    </main>

    <!-- 加载遮罩 -->
    <div class="loading-overlay" v-if="loading">
      <div class="loading-spinner"></div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { StartServer, StopServer, GetStatus, GetPrinters, GetConfig, SaveConfig, GetTasks, ClearTasks, Quit } from '../wailsjs/go/main/App'
import { EventsOn, EventsOff } from '../wailsjs/runtime/runtime'

// 状态
const loading = ref(false)
const status = ref({
  isRunning: false,
  port: 11211,
  clients: 0,
  printers: [],
  tasks: []
})

const config = ref({
  port: 11211,
  autoStart: true,
  minimizeTray: true,
  defaultPrinter: ''
})

const printers = ref([])
const tasks = ref([])

// 窗口关闭处理
async function handleWindowClose(e) {
  console.log('窗口正在关闭...')
  // 清理定时器
  if (refreshInterval) {
    clearInterval(refreshInterval)
    refreshInterval = null
  }
  // 移除事件监听
  EventsOff('status-change')
  EventsOff('task-complete')
  EventsOff('task-error')
  // 调用后端退出方法
  try {
    await Quit()
  } catch (err) {
    console.error('退出失败:', err)
  }
}

// 启动服务
async function startServer() {
  loading.value = true
  try {
    await StartServer()
    await refreshStatus()
  } catch (err) {
    console.error('启动服务失败:', err)
    alert('启动服务失败: ' + err)
  } finally {
    loading.value = false
  }
}

// 停止服务
async function stopServer() {
  loading.value = true
  try {
    await StopServer()
    await refreshStatus()
  } catch (err) {
    console.error('停止服务失败:', err)
    alert('停止服务失败: ' + err)
  } finally {
    loading.value = false
  }
}

// 获取状态
async function refreshStatus() {
  try {
    const result = await GetStatus()
    status.value = result
  } catch (err) {
    console.error('获取状态失败:', err)
  }
}

// 获取打印机列表
async function refreshPrinters() {
  try {
    const result = await GetPrinters()
    printers.value = result || []
  } catch (err) {
    console.error('获取打印机列表失败:', err)
  }
}

// 获取配置
async function loadConfig() {
  try {
    const result = await GetConfig()
    config.value = result
  } catch (err) {
    console.error('获取配置失败:', err)
  }
}

// 保存配置
async function saveConfig() {
  try {
    // 如果启用了通知，请求权限
    if (config.value.notifications && 'Notification' in window && Notification.permission !== 'granted') {
      await Notification.requestPermission()
    }
    await SaveConfig(config.value)
  } catch (err) {
    console.error('保存配置失败:', err)
  }
}

// 获取任务列表
async function refreshTasks() {
  try {
    const result = await GetTasks()
    tasks.value = result || []
  } catch (err) {
    console.error('获取任务列表失败:', err)
  }
}

// 清空任务
async function clearTasks() {
  try {
    await ClearTasks()
    tasks.value = []
  } catch (err) {
    console.error('清空任务失败:', err)
  }
}

// 格式化文件类型
function getFileTypeLabel(type) {
  const labels = {
    pdf: 'PDF',
    image: '图片',
    jpg: 'JPG',
    jpeg: 'JPEG',
    png: 'PNG',
    word: 'Word',
    doc: 'Word',
    docx: 'Word',
    excel: 'Excel',
    xls: 'Excel',
    xlsx: 'Excel'
  }
  return labels[type] || type?.toUpperCase?.() || '未知'
}

// 格式化状态
function getStatusLabel(status) {
  const labels = {
    pending: '等待中',
    processing: '处理中',
    completed: '已完成',
    failed: '失败'
  }
  return labels[status] || status
}

// 格式化时间
function formatTime(time) {
  if (!time) return '-'
  const date = new Date(time)
  return date.toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  })
}

// 显示系统通知
function showNotification(title, body, type = 'info') {
  // 检查浏览器是否支持通知
  if (!('Notification' in window)) {
    console.warn('浏览器不支持系统通知')
    return
  }
  
  // 检查通知权限
  if (Notification.permission === 'granted') {
    new Notification(title, {
      body: body,
      icon: type === 'success' ? '✅' : type === 'error' ? '❌' : 'ℹ️'
    })
  } else if (Notification.permission !== 'denied') {
    // 请求通知权限
    Notification.requestPermission().then(permission => {
      if (permission === 'granted') {
        new Notification(title, {
          body: body,
          icon: type === 'success' ? '✅' : type === 'error' ? '❌' : 'ℹ️'
        })
      }
    })
  }
}

// 定时刷新句柄
let refreshInterval = null

// 初始化
onMounted(async () => {
  // 监听窗口关闭事件
  window.addEventListener('beforeunload', handleWindowClose)
  
  // 先注册事件监听器，再调用 API
  EventsOn('status-change', (data) => {
    status.value = data
    // 同时更新打印机列表
    if (data.printers && data.printers.length > 0) {
      printers.value = data.printers
    }
  })
  
  EventsOn('task-complete', (data) => {
    refreshTasks()
    // 显示通知
    if (config.value.notifications) {
      showNotification('打印完成', `任务 ${data?.taskId || ''} 已成功打印`, 'success')
    }
  })
  
  EventsOn('task-error', (data) => {
    refreshTasks()
    // 显示通知
    if (config.value.notifications) {
      showNotification('打印失败', `任务 ${data?.taskId || ''} 失败: ${data?.error || '未知错误'}`, 'error')
    }
  })

  // 然后加载数据
  await loadConfig()
  await refreshStatus()
  await refreshPrinters()
  await refreshTasks()

  // 定时刷新
  refreshInterval = setInterval(() => {
    refreshStatus()
    refreshTasks()
  }, 3000)
})

onUnmounted(() => {
  // 移除窗口关闭监听
  window.removeEventListener('beforeunload', handleWindowClose)
  
  if (refreshInterval) {
    clearInterval(refreshInterval)
  }
  EventsOff('status-change')
  EventsOff('task-complete')
  EventsOff('task-error')
})
</script>

<style scoped>
.app-container {
  height: 100%;
  display: flex;
  flex-direction: column;
  background: var(--bg-color);
}

/* 顶部导航栏 */
.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 24px;
  height: 60px;
  background: var(--card-bg);
  border-bottom: 1px solid var(--border-color);
}

.header-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.logo {
  width: 28px;
  height: 28px;
  color: var(--primary-color);
}

.title {
  font-size: 18px;
  font-weight: 600;
}

.header-right {
  display: flex;
  align-items: center;
  gap: 16px;
}

.version {
  font-size: 12px;
  color: var(--text-secondary);
}

/* 主内容区 */
.main {
  flex: 1;
  display: flex;
  padding: 20px;
  gap: 20px;
  overflow: hidden;
}

/* 侧边栏 */
.sidebar {
  width: 320px;
  display: flex;
  flex-direction: column;
  gap: 16px;
  flex-shrink: 0;
}

/* 内容区 */
.content {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.task-card {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.task-card .card-header {
  flex-shrink: 0;
}

.task-list {
  flex: 1;
  overflow: auto;
}

/* 表单 */
.form-group {
  margin-bottom: 16px;
}

.form-group label {
  display: block;
  margin-bottom: 6px;
  font-size: 13px;
  color: var(--text-secondary);
}

.checkbox-group label {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
}

.checkbox-group input {
  width: 16px;
  height: 16px;
  cursor: pointer;
}

.button-group {
  display: flex;
  gap: 12px;
  margin-top: 20px;
}

.button-group .btn {
  flex: 1;
}

/* 打印机列表 */
.printer-list {
  max-height: 200px;
  overflow: auto;
}

.printer-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px;
  border-radius: 6px;
  transition: background 0.2s;
}

.printer-item:hover {
  background: var(--bg-color);
}

.printer-icon {
  font-size: 24px;
}

.printer-info {
  flex: 1;
  min-width: 0;
}

.printer-name {
  font-size: 14px;
  font-weight: 500;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.printer-status {
  font-size: 12px;
  color: var(--text-secondary);
}

/* 连接信息 */
.connection-info {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.info-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.info-label {
  font-size: 12px;
  color: var(--text-secondary);
}

.info-value {
  font-size: 13px;
}

code {
  padding: 2px 6px;
  background: #f5f5f5;
  border-radius: 4px;
  font-family: 'Monaco', 'Menlo', monospace;
  font-size: 12px;
}

/* 加载遮罩 */
.loading-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(255, 255, 255, 0.8);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.loading-spinner {
  width: 40px;
  height: 40px;
  border: 3px solid var(--border-color);
  border-top-color: var(--primary-color);
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}
</style>
