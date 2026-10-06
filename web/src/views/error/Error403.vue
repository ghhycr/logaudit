<template>
  <div class="err">
    <div class="code">403</div>
    <div class="msg">无权访问该页面（角色权限不足，已记录审计）</div>
    <el-button type="primary" @click="$router.push('/dashboard')">返回首页</el-button>
  </div>
</template>

<style scoped>
.err { height: 100%; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 12px; background: #fff; border-radius: 10px; }
.code { font-size: 64px; font-weight: 700; color: #ea6668; }
.msg { font-size: 14px; color: #6b7280; }
</style>
