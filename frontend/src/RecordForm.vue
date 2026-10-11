<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { Plus, Trash2, X } from 'lucide-vue-next'
import { api, money } from './api'
import type { CRMRecord, Field, LineItem, Module, SubtableRow, Value } from './types'
import ReferenceInput from './ReferenceInput.vue'
import { compressProductImage } from './productImages'

const props = defineProps<{ module: Module; record?: CRMRecord; initial?: Record<string, Value> }>()
const emit = defineEmits<{ close: []; saved: [record: CRMRecord] }>()
const form = ref<Record<string, Value>>({}), items = ref<LineItem[]>([]), error = ref(''), busy = ref(false)
const subtables = ref<Record<string, SubtableRow[]>>({})
const productImages = ref<{ file: File; url: string }[]>([]), savedProduct = ref<CRMRecord>()
const imageTypes = ['image/jpeg', 'image/png', 'image/webp', 'image/gif']
function clearProductImages() { productImages.value.forEach(image => URL.revokeObjectURL(image.url)); productImages.value = [] }
onUnmounted(clearProductImages)
function itemAmount(item: LineItem) { const base = Number(item.quantity) * Number(item.unit_price); const discounted = base * (1 - Number(item.discount_rate || 0) / 100); return Math.round(discounted * (1 + Number(item.tax_rate || 0) / 100) * 100) / 100 }
const itemTotal = computed(() => items.value.reduce((sum, item) => sum + itemAmount(item), 0))
onMounted(() => {
  for (const field of props.module.fields) {
    const saved = props.record?.data[field.key] ?? props.initial?.[field.key]
    form.value[field.key] = saved ?? (field.type === 'select' ? field.options?.[0] ?? '' : '')
    if (field.type === 'items') items.value = saved ? JSON.parse(JSON.stringify(saved)) : []
    if (field.type === 'subtable') subtables.value[field.key] = saved ? JSON.parse(JSON.stringify(saved)) : []
  }
})
function addItem() { items.value.push({ product_id: '', specification: '', quantity: '1', unit_price: '0', discount_rate: '0', tax_rate: '0' }) }
function selectProduct(index: number, product: CRMRecord) { items.value[index].unit_price = String(product.data.price); items.value[index].name = String(product.data.name) }
function addSubtableRow(field: Field) { subtables.value[field.key].push(Object.fromEntries((field.columns || []).map(column => [column.key, '']))) }
function chooseProductImages(event: Event) {
  const input = event.target as HTMLInputElement
  const files = Array.from(input.files || [])
  if (files.length > 3 || files.some(file => !imageTypes.includes(file.type) || file.size > 5 * 1024 * 1024 || !file.size)) {
    clearProductImages()
    error.value = '每次最多选择 3 张 JPEG、PNG、WebP 或 GIF 图片，单张不超过 5 MB'
  } else if (files.length) { clearProductImages(); productImages.value = files.map(file => ({ file, url: URL.createObjectURL(file) })); error.value = '' }
  input.value = ''
}
function removeProductImage(index: number) {
  const [image] = productImages.value.splice(index, 1)
  if (image) URL.revokeObjectURL(image.url)
}
async function save() {
  if (busy.value) return
  busy.value = true; error.value = ''
  let recordSaved = false
  try {
    const data = Object.fromEntries(Object.entries(form.value).filter(([key]) => { const field = props.module.fields.find(item => item.key === key); return !field?.read_only && field?.type !== 'formula' }))
    if (props.module.fields.some(f => f.type === 'items')) data.items = items.value
    for (const field of props.module.fields.filter(field => field.type === 'subtable')) data[field.key] = subtables.value[field.key]
    const previous = savedProduct.value || props.record
    const record = await api<CRMRecord>(`/records/${props.module.key}${previous ? '/' + previous.id : ''}`, previous ? 'PUT' : 'POST', { data, ...(previous ? { version: previous.version } : {}) })
    recordSaved = true
    if (props.module.key === 'products') savedProduct.value = record
    if (props.module.key === 'products' && productImages.value.length) {
      const images = new FormData()
      const files = await Promise.all(productImages.value.map(image => compressProductImage(image.file)))
      files.forEach(file => images.append('files', file))
      const response = await fetch(`/api/records/products/${record.id}/attachments`, { method: 'POST', credentials: 'same-origin', body: images })
      if (!response.ok) throw new Error((await response.json().catch(() => ({}))).detail || '图片上传失败')
      clearProductImages()
    }
    emit('saved', record)
  } catch (e) { error.value = (recordSaved ? '产品已保存，但图片上传失败：' : '') + (e as Error).message }
  finally { busy.value = false }
}
</script>

<template>
  <div class="modal-backdrop" @click.self="!busy && emit('close')">
    <section class="modal record-form" role="dialog" aria-modal="true" :aria-label="(record ? '编辑' : '新建') + module.label">
      <header class="modal-header"><div><small>{{ module.group }}</small><h2>{{ record ? '编辑' : '新建' }}{{ module.label }}</h2></div><button class="icon-button" :disabled="busy" aria-label="关闭" @click="emit('close')"><X :size="21"/></button></header>
      <form @submit.prevent="save">
        <div class="modal-body form-grid">
          <template v-for="(field, index) in module.fields" :key="field.key">
            <h3 v-if="field.group && (index === 0 || field.group !== module.fields[index - 1].group)" class="form-section-title full">{{ field.group }}</h3>
            <div v-if="field.type === 'items'" class="field full">
              <div class="section-title"><label>产品明细 <em>*</em></label><button type="button" class="text-button" @click="addItem"><Plus :size="15"/>添加产品</button></div>
              <div v-if="!items.length" class="empty small">添加产品后，系统将自动计算总金额</div>
              <div v-for="(item, index) in items" :key="index" class="line-item">
                <ReferenceInput v-model="item.product_id" module="products" required @select="selectProduct(index, $event)"/>
                <label>数量<input v-model="item.quantity" aria-label="产品数量" type="number" min="0.001" step="0.001" required/></label>
                <label>单价<input v-model="item.unit_price" aria-label="产品单价" type="number" min="0" step="0.01" required/></label><label>规格<input v-model.trim="item.specification" aria-label="产品规格" maxlength="100" placeholder="可选"/></label><label>折扣率 %<input v-model="item.discount_rate" aria-label="折扣率" type="number" min="0" max="100" step="0.01"/></label><label>税率 %<input v-model="item.tax_rate" aria-label="税率" type="number" min="0" max="100" step="0.01"/></label><span class="line-item-amount">{{ money(itemAmount(item)) }}</span>
                <button type="button" class="icon-button danger" aria-label="移除产品" @click="items.splice(index, 1)"><Trash2 :size="17"/></button>
              </div>
              <div class="item-total">合计金额 <strong>{{ money(itemTotal) }}</strong></div>
            </div>
            <div v-else-if="field.type === 'subtable'" class="field full">
              <div class="section-title"><label>{{ field.label }}</label><button type="button" class="text-button" @click="addSubtableRow(field)"><Plus :size="15"/>添加一行</button></div>
              <div v-if="!subtables[field.key]?.length" class="empty small">暂无明细</div>
              <div v-for="(row, rowIndex) in subtables[field.key]" :key="rowIndex" class="line-item">
                <label v-for="column in field.columns || []" :key="column.key">{{ column.label }}
                  <select v-if="column.type === 'select'" v-model="row[column.key]" :required="column.required"><option v-if="!column.required" value="">请选择</option><option v-for="option in column.options" :key="option" :value="option">{{ option }}</option></select>
                  <input v-else v-model="row[column.key]" :type="['number','money'].includes(column.type) ? 'number' : column.type === 'date' ? 'date' : 'text'" :step="column.type === 'money' ? '0.01' : column.type === 'number' ? 'any' : undefined" :required="column.required"/>
                </label>
                <button type="button" class="icon-button danger" aria-label="删除明细行" @click="subtables[field.key].splice(rowIndex, 1)"><Trash2 :size="17"/></button>
              </div>
            </div>
            <label v-else class="field" :class="{ full: field.type === 'textarea' }">
              <span>{{ field.label }} <em v-if="field.required">*</em></span>
              <input v-if="field.type === 'formula'" :value="String(form[field.key] || '保存后自动计算')" disabled/>
              <ReferenceInput v-else-if="field.type === 'reference'" :model-value="String(form[field.key] || '')" :module="field.module!" :required="field.required" :disabled="field.read_only" @update:model-value="form[field.key] = $event"/>
              <select v-else-if="field.type === 'select'" v-model="form[field.key]" :required="field.required" :disabled="field.read_only"><option v-if="!field.required" value="">请选择</option><option v-for="value in field.options" :key="value" :value="value">{{ value }}</option></select>
              <textarea v-else-if="field.type === 'textarea'" v-model="form[field.key] as string" rows="3" maxlength="5000" :required="field.required" :disabled="field.read_only" :placeholder="'请输入' + field.label"/>
              <input v-else v-model="form[field.key]" :type="field.type === 'money' || field.type === 'number' ? 'number' : field.type === 'date' ? 'date' : field.type === 'email' ? 'email' : 'text'" :step="field.type === 'money' ? '0.01' : 'any'" :min="field.type === 'money' ? 0 : field.min" :max="field.max" :required="field.required" :disabled="field.read_only" :placeholder="'请输入' + field.label" maxlength="300"/>
            </label>
          </template>
          <div v-if="module.key === 'products'" class="field full product-image-input"><span>产品图片（可选）</span><div class="product-image-upload"><label><span>{{ productImages.length ? '已选择 ' + productImages.length + ' 张，点击重新选择' : '选择图片（最多 3 张，单张不超过 5 MB）' }}</span><input type="file" accept="image/jpeg,image/png,image/webp,image/gif" multiple :disabled="busy" @change="chooseProductImages"/></label><span class="product-image-upload-note">保存产品时上传</span></div><div v-if="productImages.length" class="product-image-grid"><div v-for="(image, index) in productImages" :key="image.url" class="product-image-card"><img :src="image.url" :alt="image.file.name"/><footer><span :title="image.file.name">{{ image.file.name }}</span><button type="button" class="icon-button danger" :disabled="busy" :aria-label="'移除图片' + image.file.name" @click="removeProductImage(index)"><Trash2 :size="15"/></button></footer></div></div></div>
          <p v-if="error" class="error-banner full" role="alert">{{ error }}</p>
        </div>
        <footer class="modal-footer"><span v-if="module.approvable" class="muted">保存为草稿，提交审批后生效</span><button type="button" class="button" :disabled="busy" @click="emit('close')">取消</button><button class="button primary" :disabled="busy">{{ busy ? '保存中…' : '保存' + module.label }}</button></footer>
      </form>
    </section>
  </div>
</template>
