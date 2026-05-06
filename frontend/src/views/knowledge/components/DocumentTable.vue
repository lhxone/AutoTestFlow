<template>
  <a-table :columns="columns" :data-source="documents" :loading="loading" row-key="id" size="middle" :pagination="false">
    <template #bodyCell="{ column, record }">
      <template v-if="column.key === 'status'">
        <a-tag :color="statusColor(record.status)">{{ statusText(record.status) }}</a-tag>
      </template>
      <template v-if="column.key === 'action'">
        <a-space>
          <a-button size="small" :loading="actionId === record.id" @click="$emit('rebuild', record.id)">重建</a-button>
          <a-popconfirm title="确认删除该文档?" @confirm="$emit('delete', record.id)">
            <a-button size="small" danger>删除</a-button>
          </a-popconfirm>
        </a-space>
      </template>
    </template>
  </a-table>
</template>

<script setup lang="ts">
import type { KnowledgeDocument } from '@/api/knowledge'

defineProps<{ documents: KnowledgeDocument[]; loading: boolean; actionId?: number }>()
defineEmits<{ (e: 'rebuild', id: number): void; (e: 'delete', id: number): void }>()

const columns = [
  { title: '标题', dataIndex: 'title', key: 'title' },
  { title: '来源', dataIndex: 'source_type', key: 'source_type', width: 100 },
  { title: '大小', dataIndex: 'content_size', key: 'content_size', width: 100 },
  { title: 'Chunks', dataIndex: 'chunk_count', key: 'chunk_count', width: 100 },
  { title: '状态', dataIndex: 'status', key: 'status', width: 120 },
  { title: '操作', key: 'action', width: 150 },
]

function statusColor(status: string) {
  return status === 'indexed' ? 'success' : status === 'failed' ? 'error' : status === 'parsing' ? 'processing' : 'default'
}

function statusText(status: string) {
  const map: Record<string, string> = { pending: '待处理', parsing: '解析中', indexed: '已索引', failed: '失败' }
  return map[status] || status
}
</script>
