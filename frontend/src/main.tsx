import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import './App.css'
import App from './App.tsx'
import { AuthCallback } from './components/AuthCallback.tsx'

// No router dependency for a single route — Vite's dev server (and Firebase
// Hosting's SPA rewrite in prod) already falls back to index.html for any
// unknown path.
const isAuthCallback = window.location.pathname === '/auth/callback'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    {isAuthCallback ? <AuthCallback /> : <App />}
  </StrictMode>,
)
