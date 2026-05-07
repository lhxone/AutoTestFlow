<template>
  <a-form layout="vertical" :model="form" class="kb-settings">
    <a-row :gutter="16">
      <a-col :xs="24" :md="8">
        <a-form-item label="启用 RAG">
          <a-switch v-model:checked="form.enabled" />
        </a-form-item>
      </a-col>
      <a-col :xs="24" :md="8">
        <a-form-item label="Milvus Host">
          <a-input v-model:value="form.vector_store_host" />
        </a-form-item>
      </a-col>
      <a-col :xs="24" :md="8">
        <a-form-item label="Milvus Port">
          <a-input-number v-model:value="form.vector_store_port" class="full" />
        </a-form-item>
      </a-col>
      <a-col :xs="24" :md="12">
        <a-form-item label="Collection">
          <a-input v-model:value="form.vector_store_collection" />
        </a-form-item>
      </a-col>
      <a-col :xs="24" :md="12">
        <a-form-item label="Embedding Provider">
          <a-select v-model:value="form.embedding_provider">
            <a-select-option value="openai_compatible">OpenAI Compatible</a-select-option>
            <a-select-option value="zhipu">智谱 GLM</a-select-option>
            <a-select-option value="custom">Custom</a-select-option>
          </a-select>
        </a-form-item>
      </a-col>
      <a-col :xs="24" :md="12">
        <a-form-item label="Embedding Base URL">
          <a-input
            v-model:value="form.embedding_base_url"
            placeholder="https://api.openai.com/v1 或 https://open.bigmodel.cn/api/paas/v4"
          />
          <div class="field-hint">OpenAI 兼容客户端会自动请求 /embeddings，智谱填写到 /v4 即可。</div>
        </a-form-item>
      </a-col>
      <a-col :xs="24" :md="12">
        <a-form-item label="Embedding API Key">
          <a-input-password v-model:value="form.embedding_api_key" autocomplete="new-password" />
        </a-form-item>
      </a-col>
      <a-col :xs="24" :md="12">
        <a-form-item label="Embedding Model">
          <a-input v-model:value="form.embedding_model" />
        </a-form-item>
      </a-col>
      <a-col :xs="24" :md="8">
        <a-form-item label="Embedding Dimension">
          <a-input-number v-model:value="form.embedding_dimension" class="full" />
        </a-form-item>
      </a-col>
      <a-col :xs="24" :md="8">
        <a-form-item label="Batch Size">
          <a-input-number v-model:value="form.embedding_batch_size" :min="1" :max="64" class="full" />
        </a-form-item>
      </a-col>
      <a-col :xs="24" :md="8">
        <a-form-item label="Top K">
          <a-input-number v-model:value="form.top_k" class="full" />
        </a-form-item>
      </a-col>
      <a-col :xs="24" :md="8">
        <a-form-item label="Chunk Size">
          <a-input-number v-model:value="form.chunk_size" class="full" />
        </a-form-item>
      </a-col>
      <a-col :xs="24" :md="8">
        <a-form-item label="Chunk Overlap">
          <a-input-number v-model:value="form.chunk_overlap" class="full" />
        </a-form-item>
      </a-col>
      <a-col :xs="24" :md="8">
        <a-form-item label="Similarity Threshold">
          <a-input-number v-model:value="form.similarity_threshold" :min="0" :max="1" :step="0.01" class="full" />
        </a-form-item>
      </a-col>
    </a-row>
    <a-button type="primary" :loading="saving" @click="save">保存配置</a-button>
  </a-form>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { message } from 'ant-design-vue'
import { getKnowledgeConfig, saveKnowledgeConfig, type KnowledgeBaseConfig } from '@/api/knowledge'

const saving = ref(false)
const form = reactive<KnowledgeBaseConfig>({
  enabled: false,
  vector_store_type: 'milvus',
  vector_store_host: 'localhost',
  vector_store_port: 19530,
  vector_store_collection: 'autotestflow_knowledge',
  embedding_provider: 'openai_compatible',
  embedding_api_key: '',
  embedding_base_url: 'https://api.openai.com/v1',
  embedding_model: 'text-embedding-3-small',
  embedding_dimension: 1536,
  embedding_batch_size: 16,
  chunk_size: 500,
  chunk_overlap: 50,
  top_k: 5,
  similarity_threshold: 0.75,
})

async function load() {
  const res = await getKnowledgeConfig()
  Object.assign(form, res.data.data || {})
}

async function save() {
  saving.value = true
  try {
    await saveKnowledgeConfig(form)
    message.success('配置已保存')
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.kb-settings {
  max-width: 980px;
}
.full {
  width: 100%;
}
.field-hint {
  margin-top: 6px;
  color: #8c8c8c;
  font-size: 12px;
  line-height: 1.5;
}
</style>
