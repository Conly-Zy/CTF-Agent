export interface ChallengeTemplate {
  id: string
  name: string
  type: 'web' | 'pwn' | 'crypto' | 'reverse'
  description: string
  template: string
  targetPlaceholder?: string
  tips?: string[]
}

export const challengeTemplates: ChallengeTemplate[] = [
  // Web templates
  {
    id: 'web-sqli',
    name: 'SQL 注入',
    type: 'web',
    description: 'SQL 注入漏洞利用',
    template: '这是一个 SQL 注入挑战。目标网站存在 SQL 注入漏洞，需要获取数据库中的 flag。\n\n请分析目标网站，找到注入点并提取数据。',
    targetPlaceholder: 'http://challenge.example.com/login',
    tips: ['尝试 UNION SELECT 注入', '检查是否有 WAF 过滤', '尝试盲注或时间盲注'],
  },
  {
    id: 'web-xss',
    name: 'XSS 跨站脚本',
    type: 'web',
    description: '跨站脚本攻击',
    template: '这是一个 XSS 挑战。目标网站存在 XSS 漏洞，需要通过注入 JavaScript 代码获取 flag。',
    targetPlaceholder: 'http://challenge.example.com/search',
    tips: ['检查输入过滤和编码', '尝试存储型或反射型 XSS', '使用 CSP bypass 技术'],
  },
  {
    id: 'web-ssrf',
    name: 'SSRF 服务端请求伪造',
    type: 'web',
    description: '服务端请求伪造漏洞',
    template: '这是一个 SSRF 挑战。目标服务器存在 SSRF 漏洞，需要通过构造请求访问内部服务获取 flag。',
    targetPlaceholder: 'http://challenge.example.com/fetch',
    tips: ['尝试 file:// 和 gopher:// 协议', '检查是否有 IP 限制', '尝试 DNS 重绑定'],
  },
  {
    id: 'web-deserial',
    name: '反序列化漏洞',
    type: 'web',
    description: '反序列化漏洞利用',
    template: '这是一个反序列化漏洞挑战。目标应用存在不安全的反序列化，需要构造恶意序列化数据获取 flag。',
    targetPlaceholder: 'http://challenge.example.com/api',
    tips: ['识别序列化格式 (PHP/Java/Python)', '查找可用的 POP 链', '使用 ysoserial 等工具'],
  },

  // Pwn templates
  {
    id: 'pwn-buffer',
    name: '缓冲区溢出',
    type: 'pwn',
    description: '基础缓冲区溢出',
    template: '这是一个缓冲区溢出挑战。程序存在栈溢出漏洞，需要控制程序执行流程获取 flag。',
    tips: ['使用 pattern 确定偏移量', '检查安全保护 (checksec)', '寻找可用的 gadget'],
  },
  {
    id: 'pwn-format',
    name: '格式化字符串漏洞',
    type: 'pwn',
    description: '格式化字符串漏洞利用',
    template: '这是一个格式化字符串漏洞挑战。程序存在格式化字符串漏洞，需要利用它读取或写入内存获取 flag。',
    tips: ['使用 %p 泄露栈内容', '使用 %n 写入任意地址', '计算参数偏移量'],
  },
  {
    id: 'pwn-heap',
    name: '堆漏洞',
    type: 'pwn',
    description: '堆利用技术',
    template: '这是一个堆漏洞挑战。程序存在 Use-After-Free 或 Double-Free 等堆漏洞，需要利用它们获取 flag。',
    tips: ['分析堆分配和释放逻辑', '尝试 UAF / Double Free', '利用 tcache 或 fastbin'],
  },
  {
    id: 'pwn-rop',
    name: 'ROP 链',
    type: 'pwn',
    description: 'ROP 链构造',
    template: '这是一个 ROP 链挑战。需要构造 ROP 链绕过安全保护执行系统命令获取 flag。',
    tips: ['使用 ROPgadget 查找 gadget', '构造 ret2libc 或 ret2csu', '注意栈对齐问题'],
  },

  // Crypto templates
  {
    id: 'crypto-rsa',
    name: 'RSA 密码分析',
    type: 'crypto',
    description: 'RSA 相关挑战',
    template: '这是一个 RSA 密码学挑战。已知 RSA 的部分参数，需要解密或分解获取 flag。',
    tips: ['检查是否可以分解 n', '尝试 Wiener 攻击 (小 d)', '使用 factordb.com'],
  },
  {
    id: 'crypto-aes',
    name: 'AES 密码分析',
    type: 'crypto',
    description: 'AES 相关挑战',
    template: '这是一个 AES 密码学挑战。需要分析 AES 加密的弱点获取 flag。',
    tips: ['检查 IV 是否可预测', '尝试 Padding Oracle 攻击', '分析 ECB 模式的特性'],
  },
  {
    id: 'crypto-classical',
    name: '古典密码',
    type: 'crypto',
    description: '古典密码破解',
    template: '这是一个古典密码挑战。密文使用了某种古典加密算法，需要破解获取明文 flag。',
    tips: ['尝试 Caesar/ROT13 移位', '分析字母频率', '检查是否为替换密码'],
  },
  {
    id: 'crypto-math',
    name: '数学问题',
    type: 'crypto',
    description: '数论相关挑战',
    template: '这是一个数学/数论挑战。需要利用数学知识解决密码学问题获取 flag。',
    tips: ['检查模运算性质', '使用中国剩余定理', '分析离散对数问题'],
  },

  // Reverse templates
  {
    id: 'reverse-static',
    name: '静态分析',
    type: 'reverse',
    description: '二进制静态分析',
    template: '这是一个逆向工程挑战。需要对二进制文件进行静态分析，理解程序逻辑获取 flag。',
    tips: ['使用 IDA/Ghidra 分析', '查找关键字符串', '追踪 flag 验证逻辑'],
  },
  {
    id: 'reverse-dynamic',
    name: '动态调试',
    type: 'reverse',
    description: '动态调试分析',
    template: '这是一个逆向工程挑战。需要动态调试程序，观察运行时行为获取 flag。',
    tips: ['使用 GDB/LLDB 调试', '设置断点在关键函数', '检查内存中的数据'],
  },
  {
    id: 'reverse-vm',
    name: '虚拟机保护',
    type: 'reverse',
    description: '自定义虚拟机分析',
    template: '这是一个虚拟机保护的逆向挑战。程序使用了自定义虚拟机，需要分析 VM 指令集获取 flag。',
    tips: ['识别 VM 的 dispatch loop', '分析 opcode 和操作数', '编写反汇编器'],
  },
  {
    id: 'reverse-android',
    name: 'Android 逆向',
    type: 'reverse',
    description: 'Android 应用逆向',
    template: '这是一个 Android 逆向挑战。需要分析 APK 文件，找到隐藏的 flag。',
    tips: ['使用 jadx 反编译', '检查 native 库 (.so)', '分析 SharedPreferences'],
  },
]
