/**
 * 打印服务器 SDK
 * 用于 Web 浏览器连接打印服务进行打印操作
 * 
 * 使用示例:
 * const printer = new PrinterSDK('ws://localhost:11211/ws')
 * await printer.connect()
 * await printer.print({ data: base64Data, fileType: 'pdf' })
 */

class PrinterSDK {
  constructor(url = 'ws://localhost:11211/ws') {
    this.url = url
    this.ws = null
    this.connected = false
    this.callbacks = new Map()
    this.messageQueue = []
    this.eventHandlers = {
      onConnected: null,
      onDisconnected: null,
      onTaskComplete: null,
      onTaskError: null,
      onError: null
    }
  }

  /**
   * 连接到打印服务器
   */
  connect() {
    return new Promise((resolve, reject) => {
      if (this.connected) {
        resolve()
        return
      }

      this.ws = new WebSocket(this.url)

      this.ws.onopen = () => {
        this.connected = true
        console.log('已连接到打印服务器')
        
        // 发送队列中的消息
        while (this.messageQueue.length > 0) {
          const msg = this.messageQueue.shift()
          this.ws.send(JSON.stringify(msg))
        }
        
        if (this.eventHandlers.onConnected) {
          this.eventHandlers.onConnected()
        }
        resolve()
      }

      this.ws.onclose = () => {
        this.connected = false
        console.log('已断开打印服务器')
        if (this.eventHandlers.onDisconnected) {
          this.eventHandlers.onDisconnected()
        }
      }

      this.ws.onerror = (err) => {
        console.error('WebSocket 错误:', err)
        if (this.eventHandlers.onError) {
          this.eventHandlers.onError(err)
        }
        reject(err)
      }

      this.ws.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data)
          this.handleMessage(data)
        } catch (err) {
          console.error('解析消息失败:', err)
        }
      }
    })
  }

  /**
   * 断开连接
   */
  disconnect() {
    if (this.ws) {
      this.ws.close()
      this.ws = null
      this.connected = false
    }
  }

  /**
   * 处理收到的消息
   */
  handleMessage(data) {
    // 处理回调
    if (data.taskId && this.callbacks.has(data.taskId)) {
      const callback = this.callbacks.get(data.taskId)
      this.callbacks.delete(data.taskId)
      
      if (data.success) {
        callback.resolve(data)
      } else {
        callback.reject(new Error(data.error || data.message || '未知错误'))
      }
    }

    // 处理事件
    switch (data.type) {
      case 'task-complete':
        if (this.eventHandlers.onTaskComplete) {
          this.eventHandlers.onTaskComplete(data)
        }
        break
      case 'task-error':
        if (this.eventHandlers.onTaskError) {
          this.eventHandlers.onTaskError(data)
        }
        break
    }
  }

  /**
   * 发送消息
   */
  send(message) {
    return new Promise((resolve, reject) => {
      if (!this.connected) {
        reject(new Error('未连接到服务器'))
        return
      }

      const taskId = message.taskId || this.generateTaskId()
      message.taskId = taskId

      // 保存回调
      this.callbacks.set(taskId, { resolve, reject })

      // 设置超时
      setTimeout(() => {
        if (this.callbacks.has(taskId)) {
          this.callbacks.delete(taskId)
          reject(new Error('请求超时'))
        }
      }, 30000)

      this.ws.send(JSON.stringify(message))
    })
  }

  /**
   * 生成任务ID
   */
  generateTaskId() {
    return Math.random().toString(36).substring(2, 10)
  }

  /**
   * 打印文件
   * @param {Object} options - 打印选项
   * @param {string} options.data - Base64 编码的文件数据
   * @param {string} options.fileType - 文件类型 (pdf, image, word, excel)
   * @param {string} options.printer - 打印机名称 (可选)
   * @param {string} options.taskId - 任务ID (可选)
   */
  print(options) {
    const { data, fileType, printer, taskId } = options
    
    return this.send({
      type: 'print',
      data: JSON.stringify(data),
      fileType: fileType || 'pdf',
      printer: printer || '',
      taskId: taskId
    })
  }

  /**
   * 打印 PDF
   */
  printPDF(base64Data, printer) {
    return this.print({
      data: base64Data,
      fileType: 'pdf',
      printer: printer
    })
  }

  /**
   * 打印图片
   */
  printImage(base64Data, printer) {
    return this.print({
      data: base64Data,
      fileType: 'image',
      printer: printer
    })
  }

  /**
   * 打印 Word 文档
   */
  printWord(base64Data, printer) {
    return this.print({
      data: base64Data,
      fileType: 'word',
      printer: printer
    })
  }

  /**
   * 打印 Excel 文档
   */
  printExcel(base64Data, printer) {
    return this.print({
      data: base64Data,
      fileType: 'excel',
      printer: printer
    })
  }

  /**
   * 获取任务状态
   */
  getTaskStatus(taskId) {
    return this.send({
      type: 'task',
      taskId: taskId
    })
  }

  /**
   * 获取所有任务
   */
  getTasks() {
    return this.send({
      type: 'task'
    })
  }

  /**
   * 获取打印机列表
   */
  getPrinters() {
    return this.send({
      type: 'printers'
    })
  }

  /**
   * 获取服务器状态
   */
  getStatus() {
    return this.send({
      type: 'status'
    })
  }

  /**
   * 设置事件处理器
   */
  on(event, handler) {
    if (this.eventHandlers.hasOwnProperty(event)) {
      this.eventHandlers[event] = handler
    }
    return this
  }

  /**
   * 移除事件处理器
   */
  off(event) {
    if (this.eventHandlers.hasOwnProperty(event)) {
      this.eventHandlers[event] = null
    }
    return this
  }
}

// HTTP API 客户端
class PrinterHTTPClient {
  constructor(baseUrl = 'http://localhost:11211') {
    this.baseUrl = baseUrl
  }

  /**
   * 发送请求
   */
  async request(path, options = {}) {
    const response = await fetch(`${this.baseUrl}${path}`, {
      ...options,
      headers: {
        'Content-Type': 'application/json',
        ...options.headers
      }
    })
    return response.json()
  }

  /**
   * 获取服务器状态
   */
  async getStatus() {
    return this.request('/api/status')
  }

  /**
   * 获取打印机列表
   */
  async getPrinters() {
    return this.request('/api/printers')
  }

  /**
   * 打印文件
   */
  async print(options) {
    return this.request('/api/print', {
      method: 'POST',
      body: JSON.stringify(options)
    })
  }

  /**
   * 获取任务列表
   */
  async getTasks() {
    return this.request('/api/tasks')
  }
}

// 导出
if (typeof module !== 'undefined' && module.exports) {
  module.exports = { PrinterSDK, PrinterHTTPClient }
} else {
  window.PrinterSDK = PrinterSDK
  window.PrinterHTTPClient = PrinterHTTPClient
}
