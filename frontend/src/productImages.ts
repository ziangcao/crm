const maxDimension = 2048

export async function compressProductImage(file: File): Promise<File> {
  if (!['image/jpeg', 'image/png'].includes(file.type) || file.size < 100 * 1024) return file
  try {
    const image = await createImageBitmap(file)
    try {
      const scale = Math.min(1, maxDimension / Math.max(image.width, image.height))
      const canvas = document.createElement('canvas')
      canvas.width = Math.round(image.width * scale)
      canvas.height = Math.round(image.height * scale)
      const context = canvas.getContext('2d')
      if (!context) return file
      context.drawImage(image, 0, 0, canvas.width, canvas.height)
      const type = file.type === 'image/png' ? 'image/webp' : 'image/jpeg'
      const blob = await new Promise<Blob | null>(resolve => canvas.toBlob(resolve, type, file.type === 'image/png' ? 0.95 : 0.9))
      if (!blob || blob.type !== type || blob.size >= file.size * 0.85) return file
      const name = type === 'image/webp' ? file.name.replace(/\.[^.]+$/, '') + '.webp' : file.name
      return new File([blob], name, { type, lastModified: file.lastModified })
    } finally { image.close() }
  } catch { return file }
}
