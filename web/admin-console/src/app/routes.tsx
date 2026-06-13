import { Routes, Route } from 'react-router-dom'
import { RequireAdmin } from '../auth/RequireAdmin'
import { AppShell } from './shell/AppShell'
import { SignIn } from '../pages/SignIn'
import { Callback } from '../pages/Callback'
import { Forbidden } from '../pages/Forbidden'
import { Dashboard } from '../pages/Dashboard'
import { Servers } from '../pages/Servers'
import { ServerForm } from '../pages/ServerForm'
import { ServerDetail } from '../pages/ServerDetail'
import { Audit } from '../pages/Audit'
import { Settings } from '../pages/Settings'

export function AppRoutes() {
  return (
    <Routes>
      <Route path="/signin" element={<SignIn />} />
      <Route path="/callback" element={<Callback />} />
      <Route path="/forbidden" element={<Forbidden />} />
      <Route
        element={
          <RequireAdmin>
            <AppShell />
          </RequireAdmin>
        }
      >
        <Route index element={<Dashboard />} />
        <Route path="servers" element={<Servers />} />
        <Route path="servers/new" element={<ServerForm />} />
        <Route path="servers/:id" element={<ServerDetail />} />
        <Route path="servers/:id/edit" element={<ServerForm />} />
        <Route path="audit" element={<Audit />} />
        <Route path="settings" element={<Settings />} />
      </Route>
    </Routes>
  )
}
