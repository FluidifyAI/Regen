# Fluidify Regen Frontend

React + TypeScript + Vite frontend for Fluidify Regen.

## Development

```bash
# Install dependencies
npm install

# Start dev server (http://localhost:3000)
npm run dev

# Build for production
npm run build

# Preview production build
npm run preview

# Lint
npm run lint

# Format code
npm run format
```

## Environment Variables

Copy `.env.example` to `.env` and configure:

```env
VITE_API_URL=http://localhost:8080
```

## Tech Stack

- **React 18** - UI library
- **TypeScript** - Type safety
- **Vite** - Build tool
- **TailwindCSS** - Styling
- **React Router 6** - Routing

## Project Structure

```
src/
├── api/           # API client and types
├── components/    # React components
│   ├── ui/        # Base components (Button, Badge, etc.)
│   ├── incidents/ # Domain components
│   └── layout/    # Layout components (Sidebar, etc.)
├── pages/         # Route pages
├── hooks/         # Custom React hooks
└── utils/         # Utility functions
```

## Design System

See [Epic 006 Implementation Plan](../docs/plans/2026-02-06-epic-006-frontend-dashboard.md) for details on:
- Color palette (blue-primary brand)
- Typography scale
- Component patterns
- Layout system
