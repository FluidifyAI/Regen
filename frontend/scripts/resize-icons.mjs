#!/usr/bin/env node
/**
 * Resize frontend/public/logo-icon.png to PWA icon sizes.
 * Run: npm run icons
 */
import { createRequire } from 'module'
import { fileURLToPath } from 'url'
import { dirname, join } from 'path'

const require = createRequire(import.meta.url)
const sharp = require('sharp')

const __dirname = dirname(fileURLToPath(import.meta.url))
const publicDir = join(__dirname, '..', 'public')
const src = join(publicDir, 'logo-icon.png')

const sizes = [
  { size: 192, out: 'icon-192.png' },
  { size: 512, out: 'icon-512.png' },
]

for (const { size, out } of sizes) {
  const dest = join(publicDir, out)
  await sharp(src).resize(size, size).toFile(dest)
  console.log(`Generated ${out} (${size}x${size})`)
}
