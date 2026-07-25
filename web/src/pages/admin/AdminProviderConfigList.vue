<template>
  <section class="page-stack">
    <header class="page-header">
      <div>
        <h1>平台配置</h1>
        <p class="resource-note">集中管理 DNS 平台密钥；同一平台可保存多套配置，主域创建时直接选用。</p>
      </div>
      <el-button type="primary" @click="openCreate"><KeyRound :size="17" />新增配置</el-button>
    </header>

    <div class="toolbar-row">
      <el-select v-model="filters.provider" clearable placeholder="DNS 平台" class="toolbar-control">
        <el-option v-for="provider in providers" :key="provider.key" :label="provider.label" :value="provider.key" />
      </el-select>
      <el-input v-model="filters.keyword" clearable placeholder="搜索配置名称" class="toolbar-control" />
      <el-button type="primary" @click="search">搜索</el-button>
      <el-button @click="resetFilters">重置</el-button>
    </div>

    <div class="resource-card">
      <el-table v-loading="loading" :data="items" class="responsive-table">
        <el-table-column prop="name" label="配置名称" min-width="180" />
        <el-table-column label="DNS 平台" min-width="140">
          <template #default="{ row }">
            <el-tag class="compact-tag" effect="plain">{{ providerLabel(row.provider_key) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="密钥状态" width="120">
          <template #default="{ row }">
            <el-tag class="compact-tag" :type="row.config_stored ? 'success' : 'warning'">
              {{ row.config_stored ? '已保存' : '未填写' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="domain_count" label="关联主域" width="110" />
        <el-table-column label="操作" width="118" fixed="right">
          <template #default="{ row }">
            <div class="table-actions">
              <el-button text type="primary" @click="openEdit(row)">编辑</el-button>
              <el-button text type="danger" @click="remove(row)">删除</el-button>
            </div>
          </template>
        </el-table-column>
      </el-table>
      <div v-if="total > pageSize" class="resource-pagination">
        <span>共 {{ total }} 条</span>
        <el-pagination v-model:current-page="page" :page-size="pageSize" layout="prev, pager, next" :total="total" @current-change="load" />
        <el-select v-model="pageSize" class="page-size-select" @change="changePageSize">
          <el-option label="10 条/页" :value="10" />
          <el-option label="20 条/页" :value="20" />
        </el-select>
      </div>
    </div>

    <el-dialog v-model="dialogVisible" width="min(720px, 96vw)" class="provider-config-dialog">
      <template #header>
        <div class="dialog-title">
          <span class="dialog-title-icon"><KeyRound :size="18" /></span>
          <strong>{{ form.id ? '编辑平台配置' : '新增平台配置' }}</strong>
        </div>
      </template>
      <el-form class="config-form" label-position="top">
        <div class="form-grid two-columns">
          <el-form-item label="DNS 平台">
            <el-select v-model="form.provider_key" class="full-control" :disabled="form.id > 0" @change="resetConfigFields">
              <el-option v-for="provider in providers" :key="provider.key" :label="provider.label" :value="provider.key" />
            </el-select>
          </el-form-item>
          <el-form-item label="配置名称">
            <el-input v-model="form.name" class="full-control" placeholder="例如：Cloudflare 主账号" />
          </el-form-item>
        </div>
        <el-alert
          v-if="form.id"
          type="info"
          show-icon
          :closable="false"
          title="不填写新密钥时会保留当前已保存的 DNS 配置。"
        />
        <div v-if="currentProvider" class="config-grid">
          <el-form-item v-for="field in currentProvider.fields" :key="field.name" :label="fieldLabel(field)">
            <el-input
              v-model="form.config[field.name]"
              class="full-control"
              :type="field.secret ? 'password' : 'text'"
              :show-password="field.secret"
              :placeholder="configPlaceholder(field)"
              autocomplete="off"
            />
            <p v-if="field.description" class="field-tip">{{ field.description }}</p>
          </el-form-item>
        </div>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save">保存</el-button>
      </template>
    </el-dialog>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { KeyRound } from 'lucide-vue-next'
import { apiErrorMessage } from '../../api/errors'
import {
  deleteAdminProviderConfig,
  listAdminProviderConfigsPage,
  listProviders,
  saveAdminProviderConfig,
  type AdminProviderConfig,
  type ProviderField,
  type ProviderSummary,
} from '../../api/admin'

const items = ref<AdminProviderConfig[]>([])
const providers = ref<ProviderSummary[]>([])
const loading = ref(false)
const saving = ref(false)
const dialogVisible = ref(false)
const filters = reactive({ provider: '', keyword: '' })
const page = ref(1)
const pageSize = ref(10)
const total = ref(0)
const editingConfigStored = ref(false)
const form = reactive({
  id: 0,
  provider_key: '',
  name: '',
  config: {} as Record<string, string>,
})

const currentProvider = computed(() => providers.value.find((provider) => provider.key === form.provider_key))

onMounted(load)

async function load() {
  loading.value = true
  try {
    const [configResponse, providerResponse] = await Promise.all([
      listAdminProviderConfigsPage({
        provider: filters.provider || undefined,
        keyword: filters.keyword.trim() || undefined,
        page: page.value,
        page_size: pageSize.value,
      }),
      listProviders(),
    ])
    items.value = configResponse.data.items
    total.value = configResponse.data.total
    providers.value = providerResponse.data
  } finally {
    loading.value = false
  }
}

async function search() {
  page.value = 1
  await load()
}

function resetFilters() {
  filters.provider = ''
  filters.keyword = ''
  page.value = 1
  void load()
}

function changePageSize() {
  page.value = 1
  void load()
}

function openCreate() {
  editingConfigStored.value = false
  Object.assign(form, {
    id: 0,
    provider_key: providers.value[0]?.key || '',
    name: '',
    config: {},
  })
  resetConfigFields()
  dialogVisible.value = true
}

function openEdit(row: AdminProviderConfig) {
  editingConfigStored.value = row.config_stored
  Object.assign(form, {
    id: row.id,
    provider_key: row.provider_key,
    name: row.name,
    config: {},
  })
  resetConfigFields()
  dialogVisible.value = true
}

async function save() {
  if (!form.provider_key || !form.name.trim()) {
    ElMessage.warning('请填写平台与配置名称')
    return
  }
  if (!currentProvider.value) {
    ElMessage.warning('请选择支持的 DNS 平台')
    return
  }
  // Editing with stored secrets: blank fields mean "keep existing" on the server.
  // Only require credentials when creating or when no secrets are stored yet.
  if (!form.id || !editingConfigStored.value) {
    const missing = currentProvider.value.fields.find((field) => field.required && !form.config[field.name]?.trim())
    if (missing) {
      ElMessage.warning(`请填写 ${missing.label}`)
      return
    }
  }
  saving.value = true
  try {
    await saveAdminProviderConfig({
      id: form.id || undefined,
      provider_key: form.provider_key,
      name: form.name.trim(),
      config: { ...form.config },
    })
    dialogVisible.value = false
    ElMessage.success('平台配置已保存')
    await load()
  } catch (error) {
    ElMessage.error(apiErrorMessage(error, '保存平台配置失败'))
  } finally {
    saving.value = false
  }
}

async function remove(row: AdminProviderConfig) {
  try {
    await ElMessageBox.confirm(`确认删除平台配置「${row.name}」？`, '删除平台配置', { type: 'warning' })
    await deleteAdminProviderConfig(row.id)
    ElMessage.success('平台配置已删除')
    await load()
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') {
      ElMessage.error(apiErrorMessage(error, '删除平台配置失败'))
    }
  }
}

function resetConfigFields() {
  for (const key of Object.keys(form.config)) delete form.config[key]
  for (const field of currentProvider.value?.fields || []) {
    form.config[field.name] = ''
  }
}

function fieldLabel(field: ProviderField) {
  return field.required ? `${field.label} *` : field.label
}

function configPlaceholder(field: ProviderField) {
  if (form.id) return field.secret ? '留空保留已保存密钥' : '留空保留已保存配置'
  return field.description || field.label
}

function providerLabel(key: string) {
  return providers.value.find((provider) => provider.key === key)?.label || key || '未配置'
}
</script>

<style scoped>
.config-form {
  display: grid;
  gap: 16px;
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.config-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px 18px;
}

.field-tip {
  margin: 6px 0 0;
  color: #64748b;
  font-size: 12px;
  line-height: 1.5;
}

.dialog-title {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  color: #12212a;
}

.dialog-title-icon {
  width: 30px;
  height: 30px;
  display: inline-grid;
  place-items: center;
  border-radius: 8px;
  color: #087b63;
  background: #e3fbf3;
}

@media (max-width: 720px) {
  .form-grid,
  .config-grid {
    grid-template-columns: 1fr;
  }
}
</style>
