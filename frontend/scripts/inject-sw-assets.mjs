#!/usr/bin/env node
/**
 * Post-build: read dist/.vite/manifest.json and replace the SHELL_ASSETS
 * sentinel in dist/sw.js with the real content-hashed asset URLs.
 *
 * Invoked automatically via the "postbuild" npm script.
 */
import { readFileSync, writeFileSync } from 'fs'
import { join, dirname } from 'path'
import { fileURLToPath } from 'url'

const __dirname = dirname(fileURLToPath(import.meta.url))
const distDir = join(__dirname, '..', 'dist')

// Read Vite's manifest produced by `build.manifest: true`.
const manifestPath = join(distDir, '.vite', 'manifest.json')
let manifest
try {
  manifest = JSON.parse(readFileSync(manifestPath, 'utf8'))
} catch (err) {
  console.error('inject-sw-assets: could not read manifest.json —', err.message)
  process.exit(1)
}

// Collect every hashed asset path that should be in the app-shell cache.
// We include:
//   - The hashed JS entry chunk (src/main.tsx → its output file)
//   - All CSS files referenced by the entry
//   - The root HTML (always '/')
const assets = new Set(['/'])

for (const [key, entry] of Object.entries(manifest)) {
  // The entry chunk key is usually 'src/main.tsx' or similar.
  if (entry.isEntry) {
    assets.add('/' + entry.file)
    if (entry.css) {
      for (const css of entry.css) {
        assets.add('/' + css)
      }
    }
  }
}

const assetList = JSON.stringify([...assets], null, 2)
const replacement = `const SHELL_ASSETS = ${assetList}`

// Rewrite dist/sw.js.
const swPath = join(distDir, 'sw.js')
let swSource
try {
  swSource = readFileSync(swPath, 'utf8')
} catch (err) {
  console.error('inject-sw-assets: could not read dist/sw.js —', err.message)
  process.exit(1)
}

// Replace the single-line fallback declaration (the sentinel line).
const updated = swSource.replace(/^const SHELL_ASSETS = \[.*?\]$/m, replacement)

if (updated === swSource) {
  console.warn('inject-sw-assets: sentinel line not found in dist/sw.js — skipping')
} else {
  writeFileSync(swPath, updated, 'utf8')
  console.log(`inject-sw-assets: injected ${assets.size} asset(s) into dist/sw.js`)
  for (const a of assets) console.log('  ', a)
}
